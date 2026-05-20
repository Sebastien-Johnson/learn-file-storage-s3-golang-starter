package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"

	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30 
	type ReadCloser struct {
		io.Reader
		io.Closer
	}
	rc := ReadCloser{}

	http.MaxBytesReader(w, rc, maxMemory)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not find video in db", err)
		return
	}

	if userID != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "User not author of video", err)
		return
	}
	
	videoFile, videoHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to locate video file", err)
		return
	}

	defer videoFile.Close()

	//get and check content type
	contentType, _, err := mime.ParseMediaType(videoHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video content type", err)
		return
	}
	if contentType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Missing content type for video", err)
		return
	}
	//create temp path
	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to create temp file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	//defer is lifo

	//copy video data into temp file
	if _, err = io.Copy(tempFile, videoFile); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy data into file", err)
		return
	}
	

	//reads file from beginning
	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to create temp file", err)
		return
	}

	//fill newPath with random numbers and turn to str
	newPath := make([]byte, 32)
	rand.Read(newPath)
	newPathStr := base64.RawURLEncoding.EncodeToString(newPath)
	log.Print(newPathStr)

	asRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to locate video meta data", err)
		return
	}
	pathPrefix := ""
	if asRatio == "16:9" {
		pathPrefix = "landscape/"
	} else if asRatio == "9:16" {
		pathPrefix = "portrait/"
	} else {
		pathPrefix = "other/"
	}

	s3Path := pathPrefix+newPathStr
	//s3key == path to obj in s3
	s3Key := getAssetPath(s3Path, contentType)
	//give it temp file which exist in root
	fastStartPath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to generate fast start key", err)
		return
	}
	fastStartBody, err := os.Open(fastStartPath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to open post-process path", err)
		return
	}
	//close after running
	defer os.Remove(fastStartBody.Name())
	defer fastStartBody.Close()
	//defer is lifo

	fastFileBody := io.Reader(fastStartBody)

	//puts object into s3
	_, err = cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: &cfg.s3Bucket,
		Key: &s3Key,
		Body: fastFileBody,
		ContentType: &contentType,
	})
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to store object in bucket", err)
		return
	}

	
	presignedURL := cfg.s3Bucket+","+s3Key
	video.VideoURL = &presignedURL
	presignedVideo, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to create pre-signed video", err)
		return
	}

	//s3 url to video obj in bucket: bucketName/region/aspectRatio/s3key/mediaType
	newVidURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, s3Key)

	video.VideoURL = &newVidURL
	
	cfg.db.UpdateVideo(presignedVideo)
}

//accepts path to temp file and adds .processing
func processVideoForFastStart(filePath string) (string, error) {
	newPath := filePath+".processing"
	fastStart := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", newPath)
	err := fastStart.Run()
	if err != nil {
		return "", err
	}

	return newPath, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	presignRequest, err := presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucket,
		Key: &key,
	}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}
	
	return presignRequest.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}
	params := strings.Split(*video.VideoURL, ",")
	
	if len(params) != 2 {
		return video, nil
	}
	
	presignURL, err := generatePresignedURL(cfg.s3Client, params[0], params[1], time.Minute)
	if err != nil {
		return database.Video{}, err
	}
	video.VideoURL = &presignURL
	return video, nil
}
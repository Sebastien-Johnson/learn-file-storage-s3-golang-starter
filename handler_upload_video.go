package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
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
	s3Key := getAssetPath(s3Path, contentType)

	tempFileBody := io.Reader(tempFile)

	//puts object into s3
	_, err = cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: &cfg.s3Bucket,
		Key: &s3Key,
		Body: tempFileBody,
		ContentType: &contentType,
	})
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to store object in bucket", err)
		return
	}

	newVidURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, s3Key)
	video.VideoURL = &newVidURL
	cfg.db.UpdateVideo(video)
}

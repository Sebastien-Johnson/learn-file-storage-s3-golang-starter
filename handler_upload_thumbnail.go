package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 10 << 20
	err := r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse multiple data points", err)
		return
	}

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)


	// TODO: implement the upload here
	fileData, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't get thumbnail", err)
		return
	}

	mediaType := fileHeader.Header.Get("Content-Type")

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User not author of video", err)
		return
	}

	
	contentTypeSlice := strings.Split(mediaType, "/")
	contentType := contentTypeSlice[1]
	
	videoIDStr := uuid.UUID.String(videoID)
	//create new URL with video ID and content type header
	dataURL := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, videoIDStr, contentType)
	
	//create new video file path
	thumbnailFilePath := filepath.Join(cfg.assetsRoot, videoIDStr+"."+contentType)
	//create new video file at path
	thumbnailFile, err := os.Create(thumbnailFilePath)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't create new file from path", err)
		return
	}

	//write file date to new video file location
	_, err = io.Copy(thumbnailFile, fileData)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't copy image data into file", err)
		return
	}
	//save thumnail to video struct
	video.ThumbnailURL = &dataURL
	
	//save updated video to db
	err = cfg.db.UpdateVideo(video)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

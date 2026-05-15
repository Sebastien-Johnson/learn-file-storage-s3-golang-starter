package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
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

	imgData, err := io.ReadAll(fileData)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't get thumbnail", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Couldn't get video", err)
		return
	}

	newThumbnail := thumbnail{
		data:      imgData,
		mediaType: mediaType,
	}

	videoThumbnails[videoID] = newThumbnail

	err = cfg.db.UpdateVideo(database.Video{
		ID:           videoID,
		CreatedAt:    video.CreatedAt,
		UpdatedAt:    time.Now(),
		ThumbnailURL: fmt.Sprintf("http://localhost:%d/api/thumbnails/%s", cfg.port, videoID),
		VideoURL:     video.VideoURL,
	})

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

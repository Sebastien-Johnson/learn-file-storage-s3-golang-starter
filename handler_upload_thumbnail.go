package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	// TODO: implement the upload here
	//get thumbnail data and header from request
	fileData, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't parse thumbnail", err)
		return
	}

	defer fileData.Close()

	//get content type
	mimeType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid content type", err)
		return
	}
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Missing content type for thumbnail", err)
		return
	}

	//create random string for new asset file names
	newPath := make([]byte, 32)
	rand.Read(newPath)
	newPathStr := base64.RawURLEncoding.EncodeToString(newPath)
	
	//create asset file path
	assetPath := getAssetPath(newPathStr, mimeType)
	assetDiskPath := cfg.getAssetDiskPath(assetPath)
	
	//create or update file
	newFile, err := os.Create(assetDiskPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create or update file on server", err)
	}
	defer newFile.Close()

	//copy file data into file
	if _, err = io.Copy(newFile, fileData); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy data into file", err)
		return
	}
	
	//get video from db by video ID
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't get video", err)
		return
	}

	//check if user is author of video
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User not author of video", err)
		return
	}

	//get and save thumbnail to video struct in db
	url := cfg.getAssetURL(assetPath)
	video.ThumbnailURL = &url

	//save updated video to db
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

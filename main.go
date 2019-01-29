package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	photoslibrary "google.golang.org/api/photoslibrary/v1"
)

const (
	uploadURL  string = "https://photoslibrary.googleapis.com/"
	apiVersion string = "v1"
)

var fileTypes = []string{".mp4", ".mov", ".m4v", ".avi", ".mkv", ".jpg", ".png", ".webp"}

// MediaUpload ...
type MediaUpload struct {
	name        string
	uploadToken string
}

// authenticateClient ...
func authenticateClient(clientID, clientSecret string) *http.Client {
	// create new oauth2 config
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{photoslibrary.PhotoslibraryScope},
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
	}

	// prompt user to authenticate
	stateToken := fmt.Sprintf("%x", rand.Uint64())
	authCodeURL := config.AuthCodeURL(stateToken)
	fmt.Printf("Authenticate --> %s\n\n", authCodeURL)

	// verify code and get http.Client
	var authCode string
	fmt.Print("Enter code: ")
	_, err := fmt.Scanln(&authCode)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	accesstoken, err := config.Exchange(ctx, authCode)
	if err != nil {
		log.Fatal(err)
	}
	return config.Client(ctx, accesstoken)
}

// isMedia ...
func isMedia(str string) bool {
	for _, x := range fileTypes {
		if strings.Contains(str, x) {
			return true
		}
	}
	return false
}

// findMedia ...
func findMedia() (media []string, err error) {
	thisDir, err := os.Getwd()
	if err != nil {
		return
	}
	err = filepath.Walk(thisDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && isMedia(info.Name()) {
			media = append(media, path)
		}
		return nil
	})
	if err != nil {
		return
	}
	return
}

// uploadMediaFile ...
func uploadMediaFile(file *os.File, photoClient *http.Client) MediaUpload {
	// 1. upload file, get token
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/uploads", uploadURL, apiVersion), file)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Content-type", " application/octet-stream")
	req.Header.Add("X-Goog-Upload-File-Name", path.Base(file.Name()))
	req.Header.Add("X-Goog-Upload-Protocol", "raw")
	out, err := photoClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Body.Close()

	out2, err := ioutil.ReadAll(out.Body)
	if err != nil {
		log.Fatal(err)
	}
	return MediaUpload{
		name:        path.Base(file.Name()),
		uploadToken: string(out2),
	}
}

// attachMediaUpload ...
func attachMediaUpload(item MediaUpload, photoService *photoslibrary.Service) {
	batch := photoService.MediaItems.BatchCreate(&photoslibrary.BatchCreateMediaItemsRequest{
		NewMediaItems: []*photoslibrary.NewMediaItem{
			&photoslibrary.NewMediaItem{
				Description:     item.name,
				SimpleMediaItem: &photoslibrary.SimpleMediaItem{UploadToken: item.uploadToken},
			},
		},
	})
	response, err := batch.Do()
	if err != nil {
		log.Fatal(err)
	}
	// TODO: status code error checking
	// TODO: print new media item results
	fmt.Println(response.HTTPStatusCode)
	for _, x := range response.NewMediaItemResults {
		fmt.Printf("Added %s as %s", x.MediaItem.Description, x.MediaItem.Id)
	}
}

func main() {
	fmt.Println("Starting... ")

	// authenticate
	// TODO: check for args; print message if empty
	clientID := os.Getenv("GPHOTOS_CLIENTID")
	clientSecret := os.Getenv("GPHOTOS_CLIENTSECRET")
	photoClient := authenticateClient(clientID, clientSecret)

	// create new photo service
	photoService, err := photoslibrary.New(photoClient)
	if err != nil {
		log.Fatal(err)
	}

	// find files
	mediaFiles, err := findMedia()
	if err != nil {
		log.Fatal(err)
	}

	// upload files
	// TODO: do this concurrently; multiple (1) w/ single (2)
	for _, filePath := range mediaFiles {
		// 0. prep file
		file, err := os.Open(filePath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// // 1. upload file, get token
		item := uploadMediaFile(file, photoClient)

		// 2. attach file to library via token
		attachMediaUpload(item, photoService)

	}
}

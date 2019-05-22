package main

import (
	"context"
	"encoding/json"
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

var fileTypes = []string{".mp4", ".mov", ".m4v", ".avi", ".mkv", ".jpg", ".png", ".webp", ".gif"}

// MediaUpload ...
type MediaUpload struct {
	name        string
	uploadToken string
}

// MediaResult ...
type MediaResult struct {
	ID          string
	Description string
	StatusCode  int
}

// IsMedia checks if a file is an uploadable media item.
func isMedia(str string) bool {
	for _, x := range fileTypes {
		if strings.Contains(str, x) {
			return true
		}
	}
	return false
}

// FindMedia finds all media items in the current directory.
func findMedia() ([]string, error) {
	thisDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.Walk(thisDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && isMedia(info.Name()) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// NewPhotoService creates a new google photos client to perform photo uploads.
func NewPhotoService(client *http.Client) (*photoslibrary.Service, error) {
	photoService, err := photoslibrary.New(client)
	if err != nil {
		return nil, err
	}
	return photoService, nil
}

// NewAuthenticationClient creates a new http client to facilitate photo uploads.
func NewAuthenticationClient(clientID string, clientSecret string) (*http.Client, error) {
	photoClient, err := AuthenticateClient(clientID, clientSecret)
	if err != nil {
		return nil, err
	}
	return photoClient, nil
}

// UploadMediaFile uploads a media file.
func UploadMediaFile(file string, photoClient *http.Client) (upload MediaUpload, err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()

	// 1. upload file, get token
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/uploads", uploadURL, apiVersion), f)
	if err != nil {
		return upload, fmt.Errorf("Failed to create new POST Request for File: %s --> %s", f.Name(), err.Error())
	}
	req.Header.Add("Content-type", " application/octet-stream")
	req.Header.Add("X-Goog-Upload-File-Name", path.Base(f.Name()))
	req.Header.Add("X-Goog-Upload-Protocol", "raw")
	out, err := photoClient.Do(req)
	if err != nil {
		return upload, fmt.Errorf("Failed to POST File: %s --> %s", f.Name(), err.Error())
	}
	defer out.Body.Close()

	out2, err := ioutil.ReadAll(out.Body)
	if err != nil {
		return upload, fmt.Errorf("Failed to read POST response body for file: %s --> %s", f.Name(), err.Error())
	}
	fmt.Println("File uploaded: " + f.Name())
	upload = MediaUpload{
		name:        path.Base(f.Name()),
		uploadToken: string(out2),
	}
	return
}

// AttachMediaUpload finishes an upload by attaching uploaded data to new library item.
func AttachMediaUpload(item MediaUpload, photoService *photoslibrary.Service) (result MediaResult, err error) {
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
		return
	}
	fmt.Println("File attached: " + response.NewMediaItemResults[0].MediaItem.Id)

	result = MediaResult{
		Description: response.NewMediaItemResults[0].MediaItem.Description,
		StatusCode:  response.NewMediaItemResults[0].MediaItem.HTTPStatusCode,
		ID:          response.NewMediaItemResults[0].MediaItem.Id,
	}

	return
}

// AuthenticateClient creates an authenticated client for photo upload.
func AuthenticateClient(clientID, clientSecret string) (*http.Client, error) {
	// setup
	token := &oauth2.Token{}
	ctx := context.Background()
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{photoslibrary.PhotoslibraryScope},
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
	}

	// check for token file
	tokenFile, tBool := os.LookupEnv("GPHOTOS_TOKENJSON")
	if !tBool {
		// prompt user to authenticate
		stateToken := fmt.Sprintf("%x", rand.Uint64())
		authCodeURL := config.AuthCodeURL(stateToken)
		fmt.Printf("Authenticate --> %s\n\n", authCodeURL)

		// verify code and get http.Client
		var authCode string
		fmt.Print("Enter code: ")
		_, err := fmt.Scanln(&authCode)
		if err != nil {
			log.Fatalf("Failed to read entered code: %v", err)
		}

		token, err = config.Exchange(ctx, authCode)
		if err != nil {
			log.Fatalf("Failed to authorize new token: %v", err)
		}
		saveToken(token)

	} else {
		getTokenFromFile(token, tokenFile)
	}

	return config.Client(ctx, token), nil
}

func getTokenFromFile(token *oauth2.Token, tokenFile string) error {
	file, err := os.Open(tokenFile)
	if err != nil {
		log.Fatalf("Failed to open token file: %v", err)
	}
	defer file.Close()
	json.NewDecoder(file).Decode(token)
	return nil
}

// saveTokens saves token to current directory as token.json"
func saveToken(token *oauth2.Token) {
	file, err := os.Create("token.json")
	if err != nil {
		log.Fatalf("Failed to create new token file: %v", err)
	}
	defer file.Close()
	err = json.NewEncoder(file).Encode(token)
	if err != nil {
		log.Fatalf("Failed to save to new token file: %v", err)
	}
}

func main() {
	fmt.Println("Starting... ")

	// authenticate
	// TODO: check for args; print message if empty
	clientID := os.Getenv("GPHOTOS_CLIENTID")
	clientSecret := os.Getenv("GPHOTOS_CLIENTSECRET")

	photoClient, err := NewAuthenticationClient(clientID, clientSecret)
	if err != nil {
		log.Fatal(err)
	}

	// create new photo service
	photoService, err := NewPhotoService(photoClient)
	if err != nil {
		log.Fatal(err)
	}

	// find media
	mediaFiles, err := findMedia()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d files to upload!\n", len(mediaFiles))

	for i, file := range mediaFiles {
		upload, err := UploadMediaFile(file, photoClient)
		if err != nil {
			log.Fatalf("Failed to upload media: %v", err)
		}

		retryCount := 0
		retry := true
		for retry == true && retryCount < 4 {

			result, err := AttachMediaUpload(upload, photoService)
			if err != nil {
				// failed upload
				log.Printf("Failed to attach media! Retrying %d", retryCount)
				retryCount++
			} else {
				// successful upload
				retry = false
				fmt.Printf("#%d (%d) %s ", i, result.StatusCode, result.Description)
			}
		}
		if retryCount > 4 {
			log.Printf("MAX RETRIES!! Failed to upload file: %s", file)
		}

	}

}

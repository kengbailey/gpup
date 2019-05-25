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

// MediaUpload represents a successfully uploaded media item.
type MediaUpload struct {
	name        string
	uploadToken string
}

// isMedia checks if a file is an uploadable media item.
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
	upload = MediaUpload{
		name:        path.Base(f.Name()),
		uploadToken: string(out2),
	}
	return
}

// AttachMediaUpload finishes an upload by attaching uploaded data to new library item.
func AttachMediaUpload(item MediaUpload, photoService *photoslibrary.Service) (err error) {
	batch := photoService.MediaItems.BatchCreate(&photoslibrary.BatchCreateMediaItemsRequest{
		NewMediaItems: []*photoslibrary.NewMediaItem{
			&photoslibrary.NewMediaItem{
				Description:     item.name,
				SimpleMediaItem: &photoslibrary.SimpleMediaItem{UploadToken: item.uploadToken},
			},
		},
	})
	_, err = batch.Do()
	if err != nil {
		return
	}
	return
}

// AuthenticateClient creates an authenticated client for photo upload.
func AuthenticateClient(clientID, clientSecret string) (*http.Client, error) {

	// setup
	token := &oauth2.Token{}
	var err error
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{photoslibrary.PhotoslibraryScope},
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
	}

	// check for token file; if exists load and use
	_, tBool := os.LookupEnv("GPHOTOS_TOKENJSON")
	if !tBool {
		token, err = linkAuthentication(clientID, clientSecret)
		if err != nil {
			return nil, err
		}
	} else {

		tokenFile := os.Getenv("GPHOTOS_TOKENJSON")
		token, err = getTokenFromFile(tokenFile)
		if err != nil {
			token, err = linkAuthentication(clientID, clientSecret)
			if err != nil {
				return nil, err
			}
		}
	}

	return config.Client(context.Background(), token), nil
}

// linkAuthentication performs authentication via link when token json file not found
func linkAuthentication(clientID, clientSecret string) (*oauth2.Token, error) {

	// setup
	token := &oauth2.Token{}
	var err error
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{photoslibrary.PhotoslibraryScope},
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
	}
	ctx := context.Background()

	// prompt user to authenticate
	stateToken := fmt.Sprintf("%x", rand.Uint64())
	authCodeURL := config.AuthCodeURL(stateToken)
	fmt.Printf("Authenticate --> %s\n\n", authCodeURL)

	// verify code and get http.Client
	var authCode string
	fmt.Print("Enter code: ")
	_, err = fmt.Scanln(&authCode)
	if err != nil {
		return nil, fmt.Errorf("Failed to read entered code: %v", err)
	}

	token, err = config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("Failed to authorize new token: %v", err)
	}
	err = saveToken(token) // TODO: alert user that token file isbeing saved. Tell tosave to new environment variable
	if err != nil {
		return nil, err
	}
	return token, nil
}

// getTokenFromFile retrieves the authentication token from a saved json file
func getTokenFromFile(tokenFile string) (*oauth2.Token, error) {
	token := &oauth2.Token{}
	file, err := os.Open(tokenFile)
	if err != nil {
		return token, fmt.Errorf("Failed to open token file: %v", err)
	}
	defer file.Close()
	json.NewDecoder(file).Decode(token)
	return token, nil
}

// saveTokens saves token to current directory as token.json
func saveToken(token *oauth2.Token) error {
	file, err := os.Create("token.json")
	if err != nil {
		return fmt.Errorf("Failed to create new token file: %v", err)
	}
	defer file.Close()
	err = json.NewEncoder(file).Encode(token)
	if err != nil {
		return fmt.Errorf("Failed to save to new token file: %v", err)
	}
	return nil
}

func main() {
	fmt.Print("Starting... ")

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
		fmt.Printf("%d - %s... ", i+1, filepath.Base(file))
		upload, err := UploadMediaFile(file, photoClient)
		if err != nil {
			log.Printf("Failed to upload media: %v\n", err)
		} else {
			fmt.Print("uploaded... ")

			retryCount := 0
			retry := true
			for retry == true && retryCount < 4 {

				err := AttachMediaUpload(upload, photoService)
				if err != nil {
					// failed upload
					log.Printf("Failed to attach media! Retrying %d\n", retryCount)
					retryCount++
				} else {
					// successful upload
					retry = false
					fmt.Print("finished attaching... DONE!\n")
				}
			}
			if retryCount > 4 {
				log.Printf("MAX RETRIES!! Failed to upload file: %s\n", file)
			}
		}

	}

}

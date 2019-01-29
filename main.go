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
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	photoslibrary "google.golang.org/api/photoslibrary/v1"
)

var jobs = make(chan *os.File, 10)

// kickOffJobs ...
func kickOffJobs(mediaFiles []*os.File) {
	for _, file := range mediaFiles {
		jobs <- file
	}
	close(jobs)
}

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

// UploadMediaFile ...
func UploadMediaFile(file *os.File, photoClient *http.Client) (upload MediaUpload, err error) {
	// 1. upload file, get token
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/uploads", uploadURL, apiVersion), file)
	if err != nil {
		return upload, fmt.Errorf("Failed to create new POST Request for File: %s --> %s", file.Name(), err.Error())
	}
	req.Header.Add("Content-type", " application/octet-stream")
	req.Header.Add("X-Goog-Upload-File-Name", path.Base(file.Name()))
	req.Header.Add("X-Goog-Upload-Protocol", "raw")
	out, err := photoClient.Do(req)
	if err != nil {
		return upload, fmt.Errorf("Failed to POST File: %s --> %s", file.Name(), err.Error())
	}
	defer out.Body.Close()
	defer file.Close()

	out2, err := ioutil.ReadAll(out.Body)
	if err != nil {
		return upload, fmt.Errorf("Failed to read POST response body for file: %s --> %s", file.Name(), err.Error())
	}
	fmt.Println("File uploaded: " + file.Name())
	upload = MediaUpload{
		name:        path.Base(file.Name()),
		uploadToken: string(out2),
	}
	return
}

// AttachMediaUpload ...
func AttachMediaUpload(item MediaUpload, photoService *photoslibrary.Service) MediaResult {
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
	fmt.Println("File attached: " + response.NewMediaItemResults[0].MediaItem.Id)
	return MediaResult{
		Description: response.NewMediaItemResults[0].MediaItem.Description,
		StatusCode:  response.NewMediaItemResults[0].MediaItem.HTTPStatusCode,
		ID:          response.NewMediaItemResults[0].MediaItem.Id,
	}
}

// IsMedia ...
func isMedia(str string) bool {
	for _, x := range fileTypes {
		if strings.Contains(str, x) {
			return true
		}
	}
	return false
}

// FindMedia ...
func FindMedia() (media []*os.File, err error) {
	thisDir, err := os.Getwd()
	if err != nil {
		return
	}
	var files []string
	err = filepath.Walk(thisDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && isMedia(info.Name()) {

			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return
	}
	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			log.Fatal(err)
		}
		media = append(media, file)
	}
	return
}

// worker ...
func worker(wg *sync.WaitGroup, photoClient *http.Client, photoService *photoslibrary.Service) {
	for file := range jobs {
		upload, err := UploadMediaFile(file, photoClient)
		if err != nil {
			fmt.Println(err.Error())
			// continue
		}
		result := AttachMediaUpload(upload, photoService)
		fmt.Printf("(%d) %s - %s ", result.StatusCode, result.ID, result.Description)
	}
	wg.Done()
}

const (
	uploadURL  string = "https://photoslibrary.googleapis.com/"
	apiVersion string = "v1"
)

// AuthenticateClient ...
func AuthenticateClient(clientID, clientSecret string) *http.Client {
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
func main() {
	fmt.Println("Starting... ")

	// authenticate
	// TODO: check for args; print message if empty
	clientID := os.Getenv("GPHOTOS_CLIENTID")
	clientSecret := os.Getenv("GPHOTOS_CLIENTSECRET")
	photoClient := AuthenticateClient(clientID, clientSecret)

	// create new photo service
	photoService, err := photoslibrary.New(photoClient)
	if err != nil {
		log.Fatal(err)
	}

	// find media
	mediaFiles, err := FindMedia()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d files to upload!\n", len(mediaFiles))

	// kick off jobs
	go kickOffJobs(mediaFiles)

	// kick off workers
	numWorkers := 10
	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go worker(&wg, photoClient, photoService)
	}
	wg.Wait()
}

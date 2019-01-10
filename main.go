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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	photoslibrary "google.golang.org/api/photoslibrary/v1"
)

const (
	uploadURL  string = "https://photoslibrary.googleapis.com/"
	apiVersion string = "v1"
)

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

// findMedia ...
func findMedia() (media []string, err error) {
	thisDir, err := os.Getwd()
	if err != nil {
		return
	}
	err = filepath.Walk(thisDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			// TODO: restrict file types; path.Ext()
			media = append(media, path)
		}
		return nil
	})
	return
}

func main() {
	fmt.Println("Starting... ")

	// authenticate
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
		fileName := path.Base(filePath)
		file, err := os.Open(filePath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// 1. upload file, get token
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/uploads", uploadURL, apiVersion), file)
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Add("Content-type", " application/octet-stream")
		req.Header.Add("X-Goog-Upload-File-Name", fileName)
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
		token := string(out2)

		// 2. attach file to library via token
		batch := photoService.MediaItems.BatchCreate(&photoslibrary.BatchCreateMediaItemsRequest{
			NewMediaItems: []*photoslibrary.NewMediaItem{
				&photoslibrary.NewMediaItem{
					Description:     fileName,
					SimpleMediaItem: &photoslibrary.SimpleMediaItem{UploadToken: token},
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
		// for _, x := range response.NewMediaItemResults {
		// 	fmt.Printf("Added %s as %s", x.MediaItem.Description, x.MediaItem.Id)
		// }

	}
}

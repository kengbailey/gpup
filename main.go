package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	photoslibrary "google.golang.org/api/photoslibrary/v1"
)

func main() {
	fmt.Println("Starting... ")

	// get login creds
	clientID := os.Getenv("GPHOTOS_CLIENTID")
	clientSecret := os.Getenv("GPHOTOS_CLIENTSECRET")

	// create new o-auth client
	ctx := context.Background()
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{photoslibrary.PhotoslibraryScope},
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
	}

	var n uint64
	err := binary.Read(rand.Reader, binary.LittleEndian, &n)
	if err != nil {
		log.Fatal(err)
	}
	state := fmt.Sprintf("%x", n)

	authCodeURL := config.AuthCodeURL(state)
	fmt.Printf("Authenticate --> %s\n\n", authCodeURL)

	var authCode string
	fmt.Print("Enter code: ")
	_, err = fmt.Scanln(&authCode)
	if err != nil {
		log.Fatal(err)
	}
	accesstoken, err := config.Exchange(ctx, authCode)
	if err != nil {
		log.Fatal(err)
	}
	photoClient := config.Client(ctx, accesstoken)

	// create new photos helper
	photos, err := photoslibrary.New(photoClient)
	if err != nil {
		log.Fatal(err)
	}

	// find files
	var images []string
	thisDir, _ := os.Getwd()
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			images = append(images, path)
		}
		return nil
	}
	_ = filepath.Walk(thisDir, walkFunc)

	// upload files
	for _, image := range images {
		// prep file
		fileName := path.Base(image)
		file, err := os.Open(image)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// upload file, get token
		api := "v1"
		url := "https://photoslibrary.googleapis.com/"
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/uploads", url, api), file)
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

		// attach file to library via token
		batch := photos.MediaItems.BatchCreate(&photoslibrary.BatchCreateMediaItemsRequest{
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
		fmt.Println(response.HTTPStatusCode)
		// for _, x := range response.NewMediaItemResults {
		// 	fmt.Printf("Added %s as %s", x.MediaItem.Description, x.MediaItem.Id)
		// }

	}
}

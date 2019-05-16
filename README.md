# GPup

Command line utility for uploading media to Google Photos written in Go. 
Currently supports the following file types. 
```
{".mp4", ".mov", ".m4v", ".avi", ".mkv", ".jpg", ".png", ".webp", ".gif"}
```

## Getting Started

Visit GPhotos API [authentication page](https://developers.google.com/photos/library/guides/authentication-authorization/) and follow instructions for "Authorizing requests with OAuth 2.0". Activate the Photos LIbrary API within the Google API Console and download the json credentials associated with your new client. 

Save this file to disk and create an environment variable named GPHOTOS_TOKENJSON, which holds the path to the json file. 
```
export GPHOTOS_TOKENJSON=/path/to/json/file.json
```

Clone repository
```
git clone https://github.com/kengbailey/gpup.git
```

Build 
```
go build -o gpup
```

Run executable within folder containing images to upload
```
./gpup
```

### Status

Not yet complete but working. Need to implement upload retrying functionality and then concurrent uploads. 




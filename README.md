# GPup

Command line utility for uploading media to Google Photos written in Go. 
Currently supports the following file types. 
```
{".mp4", ".mov", ".m4v", ".avi", ".mkv", ".jpg", ".png", ".webp", ".gif"}
```

## Getting Started

Visit GPhotos API [authentication page](https://developers.google.com/photos/library/guides/authentication-authorization/) and follow instructions for "Authorizing requests with OAuth 2.0". Activate the Photos LIbrary API within the Google API Console and create a new client. Save client ID and secret as environment variables. 
```
export GPHOTOS_CLIENTID=abc123qwertyID
export GPHOTOS_CLIENTSECRET=abc123qwertySECRET
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

### First Run
First run of the program will present you with a link to allow access to your google photos account. Select an account and you will be presented with token string. Copy this into the command line and continue. The program will then save the token for use later. You can now save this file to a separate location and create an environment variable pointing to the location of the saved json file for use later. When the program runs next, if this environment variable is set it will automatically log you into the same account.
```
export GPHOTOS_TOKENJSON=/path/to/json/file.json
```

### Status

Working! Log output needs to be improved. I'd like to add concurrent uploads sometime in the future. 




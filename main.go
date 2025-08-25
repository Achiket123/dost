package main

import (
	"dost/cmd/app"
	"os"
)

func main() {
	if app.Execute() != nil {
		os.Exit(1)
	}
}

/*
curl -X POST "https://generativelanguage.googleapis.com/v1beta/cachedContents?key=AIzaSyBjMl6WI3bClsTlqYRorT5PAj1120dUaQ0" \
-H 'Content-Type: application/json' \
-d @request.json \
> cache.json

curl -X POST "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-001:generateContent?key=AIzaSyBjMl6WI3bClsTlqYRorT5PAj1120dUaQ0" \
-H 'Content-Type: application/json' \
-d '{
      "contents": [
        {
          "parts":[{
            "text": "Please summarize this transcript"
          }],
          "role": "user"
        },
      ],
      "cachedContent": "'$CACHE_NAME'"
    }'
*/

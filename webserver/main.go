package main

import (
	"net/http"
)

func main() {
	http.ListenAndServe(":8081", http.FileServer(http.Dir("./docroot")))
}

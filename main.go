package main 

import (
	"log"
	"net/http"
)

func main(){
	mux := http.NewServeMux()

	s := &http.Server{
		Addr: ":8080",
		Handler: mux,
	}

	log.Printf("Server starting on %s", s.Addr)
	if err := s.ListenAndServe(); err != nil{
		log.Fatal(err)
	}
}
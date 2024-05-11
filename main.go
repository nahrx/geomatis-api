package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/nahargo/geomatis-api/api"
	"github.com/nahargo/geomatis-api/storage"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	listenAddr := flag.String("listenAddr", ":"+port, "server listen address port")
	flag.Parse()
	//var store storage.Storage
	// filename := "sls_6471_uji_coba.geojson"
	// fileData, err := ioutil.ReadFile(filename)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	store, err := storage.NewPostgreStorage()
	if err != nil {
		log.Fatal(err)
	}
	// geom, err := store.CreateMasterMaps("testing", &fileData)

	// fmt.Printf("%+v\n", string(geom))
	server := api.NewServer(*listenAddr, store)
	fmt.Println("server is running on port", *listenAddr)
	log.Fatal(server.Start())

	// size, _ := util.GetImageDimensions("64710500030025.rotate.jpg")
	// fmt.Println(size)

}

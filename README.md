# Geomatis - Automatic raster Georeferencing
Automatically georeference raster maps. The system will generate the world file as a raster georeferenced file

## World file
These are the supported world file extension in this API

| Raster extension  | World file extension |
| ------------- | ------------- |
| .jpg | .jgw |
| .jpeg | .jgw |
| .png | .pgw |

## Methodology
-   Raster image yang diupload akan diproses untuk mendapatkan koordinat sudut kotak terluas yang ada di raster file
-   Luas Koordinat kotak akan dikurangi dengan margin polygon sehingga diperoleh koordinat bounding box polygon yang ada di raster file
-   coordinates for Bounding box of raster image will be matched with real coordinates for bounding box polygon from master maps using georeferened algorithm to generate 6 parameters in a world file.
-   The world file that has been generated will be saved together with the raster file in the server.

## Feature
-	Melakukan georeferensi banyak raster file sekaligus dengan waktu yang cepat
-   Matching yang fleksibel antara properti polygon di master map dan nama file raster peta
-   Penyimpanan file hasil georeferensi yang fleksibel bisa dipisahkan berdasarkan properti yang dipilih pada master peta, misal disimpan berdasarkan kecamatan atau lebih spesifik lagi bisa disimpan berdasarkan 2 atau lebih properti, seperti kecamatan dan desa, tergantung pada properti yang dipilih sebagai grouping.
-   Mampu mendeteksi gambar raster peta yang dirotasi
-   
## Syarat yang dipenuhi pada raster peta
-   scanned raster maps yang diupload sekaligus have similar template
-   box container which contain map must be scanned properly, tidak boleh ada lipatan kertas yang menyebabkan box container tidak sempurna
-   
## Further development ideas:
-   Authentication and authorization API
-   Suggest available configuration model for georeferencing raster maps. dan Menyimpan konfigurasi yang dibuat sendiri untuk digunakan kembali
-   improve algorithm and computer vision capability for georecerencing raster maps
-   overcome unexpected EOF (Premature end of JPEG file) when decoding image file using Go image library for image processing

## Installation
-	Install postgresql, python, go, and opencv
-   Rename `backup.env` into `.env` and configure PostgreSQL database connection
-	Run the Go server

##
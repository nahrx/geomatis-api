# Geomatis - Georeferensi otomatis peta raster
API ini digunakan oleh BPS Kota Balikpapan untuk melakukan proses georeferensi scan raster peta WS dan WB hasil kegiatan survei dan sensus, sehingga proses digitalisasi peta dapat dilakukan dengan lebih cepat dan efisien. API ini Secara otomatis mengenerate world file dari peta raster yang diupload.

Sistem ini tidak terbatas hanya digunakan untuk peta WS dan WB di BPS, tapi juga bisa digunakan untuk georeferensi raster peta polygon secara umum, selama memiliki master peta polygon dalam bentuk geojson sebagai patokannya.

## Latar Belakang
Sebelumnya BPS Kota Balikpapan melakukan georeferensi peta WS dan WB secara manual satu per satu menggunakan QGIS. Proses tersebut cukup lama karena 1 peta memerlukan waktu sekitar 2 menit untuk georeferensi, sedangkan peta yang perlu digeoreferensi ada seribu lebih peta. Agar proses georeferensi lebih cepat, maka dibangunlah sistem georeferensi secara otomatis. dengan sistem ini, BPS Kota Balikpapan bisa melakukan georeferensi peta dalam jumlah ribuan dalam waktu hanya beberapa menit saja karena dibantu dengan Goroutine di GO untuk proses concurrency nya. 

## World file
Dibawah ini adalah daftar world file yang disupport dalam API ini

| Raster extension  | World file extension |
| ------------- | ------------- |
| .jpg | .jgw |
| .jpeg | .jgw |
| .png | .pgw |

## Metodologi georeferensi peta
-   Peta raster yang diupload akan diproses untuk mendapatkan koordinat sudut kotak terluas yang ada di raster menggunakan opencv
-   Luas Koordinat kotak akan dikurangi dengan margin polygon sehingga diperoleh koordinat bounding box polygon yang ada di raster
-   Koordinat bounding box dari peta raster akan dicocokkan dengan koordinat bounding box dari polygon peta digital. proses pencocokan ini menggunakan algoritma georeferensi dengan men generate 6 parameter world file.
-   world file yang terbentuk akan disimpan bersama dengan file peta raster di server yang nantinya bisa didownload

## Fitur
-	Melakukan georeferensi banyak file raster sekaligus dengan waktu yang cepat, didukung dengan Goroutine untuk concurrency.
-   Hasil georeferensi yang akurat, didukung dengan teknologi computer vision menggunakan library OpenCV 
-   Matching yang fleksibel antara properti polygon di master map dan nama file raster peta
-   Mampu mendeteksi gambar raster peta yang dirotasi
-   Penyimpanan file hasil georeferensi yang fleksibel bisa dipisahkan berdasarkan properti yang dipilih pada master peta, misal disimpan berdasarkan kecamatan atau lebih spesifik lagi bisa disimpan berdasarkan 2 atau lebih properti, seperti kecamatan dan desa, tergantung pada properti yang dipilih sebagai grouping.

## Syarat yang dipenuhi pada raster peta
-   box container yang mengandung peta harus discan secara baik, tidak boleh ada lipatan kertas yang menyebabkan box container tidak sempurna
-   
## Ide pengembangan kedepannya
-   Authentication and authorization API
-   Menyediakan konfigurasi yang bisa langsung digunakan untuk georeferensi peta raster, misal konfigurasi untuk peta WB dan WS BPS. diharapkan juga bisa menyimpan konfigurasi yang dibuat sendiri untuk digunakan kembali
-   Meningkatkan algoritma and kemampuan computer vision untuk georeferensi peta raster

## Instalasi
-	Install postgresql, python, go, and opencv
-   Rename `backup.env` into `.env` and configure PostgreSQL database connection
-	Run the Go server
-   Untuk dokumentasi API bisa dilihat di [Dokumentasi API](./assets/Dokumentasi%20API.pdf)!
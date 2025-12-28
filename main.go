package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/jung-kurt/gofpdf"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error: tidak bisa membaca file .env")
		fmt.Println("Pastikan file .env ada di direktori yang sama")
		return
	}

	// Baca konfigurasi dari .env
	baseURL := getEnv("BASE_URL", "")
	subfolder := getEnv("SUBFOLDER", "")
	sessionID := getEnv("PHPSESSID", "")
	outputName := getEnv("OUTPUT_NAME", "")
	maxPageStr := getEnv("MAX_PAGE", "")

	maxPage, _ := strconv.Atoi(maxPageStr)
	if maxPage <= 0 {
		maxPage = 200
	}

	// Validasi
	if sessionID == "" {
		fmt.Println("Error: PHPSESSID tidak boleh kosong di file .env!")
		return
	}

	// Handle subfolder trailing slash
	if !strings.HasSuffix(subfolder, "/") {
		subfolder += "/"
	}

	cookie := "PHPSESSID=" + sessionID

	// Buat folder temp untuk gambar
	tempDir := "temp_images"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		fmt.Println("Mkdir error:", err)
		return
	}

	fmt.Println("==== Konfigurasi ====")
	fmt.Printf("Base URL   : %s\n", baseURL)
	fmt.Printf("Subfolder  : %s\n", subfolder)
	fmt.Printf("Output PDF : %s.pdf\n", outputName)
	fmt.Printf("Max Page   : %d\n", maxPage)
	fmt.Printf("Cookie     : PHPSESSID=%s...\n\n", sessionID[:min(10, len(sessionID))])

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	var downloadedFiles []string
	startTime := time.Now()

	// Download semua modul secara sequential
	for module := 1; module <= 9; module++ {
		doc := fmt.Sprintf("M%d", module)
		fmt.Printf("\n=== Modul %d ===\n", module)

		consecutiveErrors := 0

		for page := 1; page <= maxPage; page++ {
			fileName := fmt.Sprintf("%s_page_%03d.jpg", doc, page)
			filePath := filepath.Join(tempDir, fileName)

			url := fmt.Sprintf(
				"%s?doc=%s&format=jpg&subfolder=%s&page=%d",
				baseURL, doc, subfolder, page,
			)

			success, stop := downloadPage(client, url, filePath, cookie, doc, page)

			if stop {
				fmt.Printf("  Modul %d selesai di halaman %d\n", module, page-1)
				break
			}

			if success {
				downloadedFiles = append(downloadedFiles, filePath)
				consecutiveErrors = 0
			} else {
				consecutiveErrors++
				if consecutiveErrors >= 5 {
					fmt.Printf("  5 error berturut-turut, lanjut ke modul berikutnya\n")
					break
				}
			}

			// Delay antar request untuk menghindari rate limit
			time.Sleep(1 * time.Second)
		}

		// Jeda antar modul
		fmt.Printf("  Jeda 3 detik sebelum modul berikutnya...\n")
		time.Sleep(3 * time.Second)
	}

	downloadTime := time.Since(startTime)
	fmt.Printf("\n==== Download selesai dalam %s ====\n", downloadTime.Round(time.Second))
	fmt.Printf("Total file: %d\n", len(downloadedFiles))

	if len(downloadedFiles) == 0 {
		fmt.Println("Tidak ada file yang berhasil didownload!")
		fmt.Println("Kemungkinan penyebab:")
		fmt.Println("  1. PHPSESSID sudah expired")
		fmt.Println("  2. Subfolder salah")
		fmt.Println("  3. IP di-block sementara")
		return
	}

	// Sort files untuk memastikan urutan benar
	sort.Strings(downloadedFiles)

	// Generate PDF
	fmt.Printf("\n==== Membuat PDF ====\n")
	pdfPath := outputName + ".pdf"

	err = createPDF(downloadedFiles, pdfPath)
	if err != nil {
		fmt.Printf("Error membuat PDF: %v\n", err)
		return
	}

	fmt.Printf("PDF berhasil dibuat: %s\n", pdfPath)

	// Hapus folder temp
	os.RemoveAll(tempDir)
	fmt.Println("File temporary dihapus.")

	totalTime := time.Since(startTime)
	fmt.Printf("\n==== SELESAI dalam %s ====\n", totalTime.Round(time.Second))
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func downloadPage(client *http.Client, url, filePath, cookie, doc string, page int) (success bool, stop bool) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("  [ERROR] %s page %d: request error\n", doc, page)
		return false, false
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  [ERROR] %s page %d: HTTP error\n", doc, page)
		return false, false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return false, true // Stop modul ini
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  [SKIP] %s page %d: status %d\n", doc, page, resp.StatusCode)
		return false, true
	}

	file, err := os.Create(filePath)
	if err != nil {
		fmt.Printf("  [ERROR] %s page %d: file create error\n", doc, page)
		return false, false
	}

	size, err := io.Copy(file, resp.Body)
	file.Close()

	if err != nil || size < 2000 {
		_ = os.Remove(filePath)
		fmt.Printf("  [SKIP] %s page %d: file kecil (%d bytes) - kemungkinan session habis\n", doc, page, size)
		return false, true // Session mungkin expired, stop
	}

	fmt.Printf("  [OK] %s page %d (%d bytes)\n", doc, page, size)
	return true, false
}

func createPDF(imagePaths []string, outputPath string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")

	for i, imgPath := range imagePaths {
		// Buka image untuk mendapatkan dimensi
		file, err := os.Open(imgPath)
		if err != nil {
			fmt.Printf("  Skip %s: %v\n", imgPath, err)
			continue
		}

		img, _, err := image.DecodeConfig(file)
		file.Close()
		if err != nil {
			fmt.Printf("  Skip %s: tidak bisa decode\n", imgPath)
			continue
		}

		// Tambah halaman baru
		pdf.AddPage()

		// Hitung ukuran untuk fit di A4 (210x297mm) dengan margin
		pageWidth := 210.0
		pageHeight := 297.0
		margin := 5.0

		availWidth := pageWidth - 2*margin
		availHeight := pageHeight - 2*margin

		imgWidth := float64(img.Width)
		imgHeight := float64(img.Height)

		// Scale to fit
		ratio := imgWidth / imgHeight
		var finalWidth, finalHeight float64

		if availWidth/availHeight > ratio {
			finalHeight = availHeight
			finalWidth = finalHeight * ratio
		} else {
			finalWidth = availWidth
			finalHeight = finalWidth / ratio
		}

		// Center image
		x := margin + (availWidth-finalWidth)/2
		y := margin + (availHeight-finalHeight)/2

		// Register dan tambahkan image
		pdf.RegisterImageOptions(imgPath, gofpdf.ImageOptions{ImageType: "JPEG", ReadDpi: true})
		pdf.Image(imgPath, x, y, finalWidth, finalHeight, false, "", 0, "")

		if (i+1)%10 == 0 {
			fmt.Printf("  Progress: %d/%d halaman\n", i+1, len(imagePaths))
		}
	}

	fmt.Printf("  Menyimpan PDF...\n")
	return pdf.OutputFileAndClose(outputPath)
}

package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime/multipart"
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
		log.Println("Error: tidak bisa membaca file .env")
		log.Println("Pastikan file .env ada di direktori yang sama")
		return
	}

	// Baca konfigurasi dari .env
	baseURL := getEnv("BASE_URL", "")
	subfolder := getEnv("SUBFOLDER", "")
	sessionID := getEnv("PHPSESSID", "")
	outputName := getEnv("OUTPUT_NAME", "")
	maxPageStr := getEnv("MAX_PAGE", "")
	userAgent := getEnv("USER_AGENT", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	referer := getEnv("REFERER", "")
	accept := getEnv("ACCEPT", "image/webp,image/apng,image/*,*/*;q=0.8")

	// WAHA WhatsApp config
	wahaAPIURL := getEnv("WAHA_API_URL", "")
	wahaAPIKey := getEnv("WAHA_API_KEY", "")
	wahaSession := getEnv("WAHA_SESSION", "default")
	wahaRecipient := getEnv("WAHA_RECIPIENT", "")

	maxPage, _ := strconv.Atoi(maxPageStr)
	if maxPage <= 0 {
		maxPage = 200
	}

	// Validasi
	if sessionID == "" {
		log.Println("Error: PHPSESSID tidak boleh kosong di file .env!")
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
		log.Println("Mkdir error:", err)
		return
	}

	log.Println("==== Konfigurasi ====")
	log.Printf("Base URL   : %s", baseURL)
	log.Printf("Subfolder  : %s", subfolder)
	log.Printf("Output PDF : %s.pdf", outputName)
	log.Printf("Max Page   : %d", maxPage)
	log.Printf("Cookie     : PHPSESSID=%s...", sessionID[:min(10, len(sessionID))])
	if wahaAPIURL != "" && wahaRecipient != "" {
		log.Printf("WhatsApp   : %s", wahaRecipient)
	} else {
		log.Println("WhatsApp   : Tidak dikonfigurasi")
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	var downloadedFiles []string
	startTime := time.Now()

	// Download semua modul secara sequential
	for module := 1; module <= 9; module++ {
		doc := fmt.Sprintf("M%d", module)
		log.Printf("=== Modul %d ===", module)

		consecutiveErrors := 0

		for page := 1; page <= maxPage; page++ {
			fileName := fmt.Sprintf("%s_page_%03d.jpg", doc, page)
			filePath := filepath.Join(tempDir, fileName)

			url := fmt.Sprintf(
				"%s?doc=%s&format=jpg&subfolder=%s&page=%d",
				baseURL, doc, subfolder, page,
			)

			success, stop := downloadPage(client, url, filePath, cookie, userAgent, referer, accept, doc, page)

			if stop {
				log.Printf("  Modul %d selesai di halaman %d", module, page-1)
				break
			}

			if success {
				downloadedFiles = append(downloadedFiles, filePath)
				consecutiveErrors = 0
			} else {
				consecutiveErrors++
				if consecutiveErrors >= 5 {
					log.Printf("  5 error berturut-turut, lanjut ke modul berikutnya")
					break
				}
			}

			// Delay antar request untuk menghindari rate limit
			time.Sleep(1 * time.Second)
		}

		// Jeda antar modul
		log.Printf("  Jeda 3 detik sebelum modul berikutnya...")
		time.Sleep(3 * time.Second)
	}

	downloadTime := time.Since(startTime)
	log.Printf("==== Download selesai dalam %s ====", downloadTime.Round(time.Second))
	log.Printf("Total file: %d", len(downloadedFiles))

	if len(downloadedFiles) == 0 {
		log.Println("Tidak ada file yang berhasil didownload!")
		log.Println("Kemungkinan penyebab:")
		log.Println("  1. PHPSESSID sudah expired")
		log.Println("  2. Subfolder salah")
		log.Println("  3. IP di-block sementara")
		return
	}

	// Sort files untuk memastikan urutan benar
	sort.Strings(downloadedFiles)

	// Generate PDF
	log.Printf("==== Membuat PDF ====")
	pdfPath := outputName + ".pdf"

	err = createPDF(downloadedFiles, pdfPath)
	if err != nil {
		log.Printf("Error membuat PDF: %v", err)
		return
	}

	log.Printf("PDF berhasil dibuat: %s", pdfPath)

	// Hapus folder temp
	os.RemoveAll(tempDir)
	log.Println("File temporary dihapus.")

	// Kirim ke WhatsApp jika dikonfigurasi
	if wahaAPIURL != "" && wahaRecipient != "" && wahaAPIKey != "" {
		log.Printf("==== Mengirim ke WhatsApp ====")
		err = sendToWhatsApp(wahaAPIURL, wahaAPIKey, wahaSession, wahaRecipient, pdfPath, outputName)
		if err != nil {
			log.Printf("Error mengirim ke WhatsApp: %v", err)
		} else {
			log.Printf("PDF berhasil dikirim ke WhatsApp: %s", wahaRecipient)
		}
	}

	totalTime := time.Since(startTime)
	log.Printf("==== SELESAI dalam %s ====", totalTime.Round(time.Second))
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func downloadPage(client *http.Client, url, filePath, cookie, userAgent, referer, accept, doc string, page int) (success bool, stop bool) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("  [ERROR] %s page %d: request error", doc, page)
		return false, false
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", referer)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Accept", accept)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("  [ERROR] %s page %d: HTTP error", doc, page)
		return false, false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return false, true // Stop modul ini
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("  [SKIP] %s page %d: status %d", doc, page, resp.StatusCode)
		return false, true
	}

	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("  [ERROR] %s page %d: file create error", doc, page)
		return false, false
	}

	size, err := io.Copy(file, resp.Body)
	file.Close()

	if err != nil || size < 2000 {
		_ = os.Remove(filePath)
		log.Printf("  [SKIP] %s page %d: file kecil (%d bytes) - kemungkinan session habis", doc, page, size)
		return false, true // Session mungkin expired, stop
	}

	log.Printf("  [OK] %s page %d (%d bytes)", doc, page, size)
	return true, false
}

func createPDF(imagePaths []string, outputPath string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")

	for i, imgPath := range imagePaths {
		// Buka image untuk mendapatkan dimensi
		file, err := os.Open(imgPath)
		if err != nil {
			log.Printf("  Skip %s: %v", imgPath, err)
			continue
		}

		img, _, err := image.DecodeConfig(file)
		file.Close()
		if err != nil {
			log.Printf("  Skip %s: tidak bisa decode", imgPath)
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
			log.Printf("  Progress: %d/%d halaman", i+1, len(imagePaths))
		}
	}

	log.Printf("  Menyimpan PDF...")
	return pdf.OutputFileAndClose(outputPath)
}

func sendToWhatsApp(apiURL, apiKey, session, recipient, filePath, fileName string) error {
	// Cek ukuran file
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("gagal membaca info file: %v", err)
	}

	fileSizeMB := float64(fileInfo.Size()) / (1024 * 1024)
	log.Printf("  Ukuran file: %.2f MB", fileSizeMB)

	// Untuk mengirim file
	log.Println("  Menggunakan multipart upload untuk file...")
	return sendToWhatsAppMultipart(apiURL, apiKey, session, recipient, filePath, fileName)

}

func sendToWhatsAppMultipart(apiURL, apiKey, session, recipient, filePath, fileName string) error {
	// Buka file untuk streaming (menghindari load seluruh file ke memory)
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("gagal membuka file: %v", err)
	}
	defer file.Close()

	// Buat pipe untuk streaming multipart data
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Channel untuk error handling
	errChan := make(chan error, 1)

	// Goroutine untuk menulis multipart data
	go func() {
		defer pw.Close()
		defer writer.Close()

		// Tambah field chatId
		if err := writer.WriteField("chatId", recipient+"@c.us"); err != nil {
			errChan <- fmt.Errorf("gagal menulis chatId: %v", err)
			return
		}

		// Tambah field caption
		caption := fmt.Sprintf("📚 %s\n\nDikirim otomatis oleh Script Download Modul", fileName)
		if err := writer.WriteField("caption", caption); err != nil {
			errChan <- fmt.Errorf("gagal menulis caption: %v", err)
			return
		}

		// Tambah file dengan streaming
		part, err := writer.CreateFormFile("file", fileName+".pdf")
		if err != nil {
			errChan <- fmt.Errorf("gagal membuat form file: %v", err)
			return
		}

		// Copy file secara streaming (tidak load seluruh file ke memory)
		if _, err := io.Copy(part, file); err != nil {
			errChan <- fmt.Errorf("gagal streaming file: %v", err)
			return
		}

		errChan <- nil
	}()

	// Buat request dengan streaming body
	url := fmt.Sprintf("%s/api/sendFile", apiURL)
	req, err := http.NewRequest("POST", url, pr)
	if err != nil {
		return fmt.Errorf("gagal membuat request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Api-Key", apiKey)

	// Add session parameter
	q := req.URL.Query()
	q.Add("session", session)
	req.URL.RawQuery = q.Encode()

	// Client dengan timeout lebih panjang untuk file besar
	client := &http.Client{Timeout: 600 * time.Second} // 10 menit timeout
	resp, err := client.Do(req)

	// Cek error dari goroutine
	writeErr := <-errChan
	if writeErr != nil {
		return writeErr
	}

	if err != nil {
		return fmt.Errorf("gagal mengirim request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

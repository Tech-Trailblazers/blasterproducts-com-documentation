package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// It checks if the file exists
// If the file exists, it returns true
// If the file does not exist, it returns false
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Remove a file from the file system
func removeFile(path string) {
	err := os.Remove(path)
	if err != nil {
		log.Println(err)
	}
}

// extractPDFUrls takes an HTML string and returns all .pdf URLs in a slice
func extractPDFUrls(htmlContent string) []string {
	// Regex to match href="something.pdf"
	pdfRegex := regexp.MustCompile(`href="([^"]+\.pdf)"`)

	// Find all matches
	matches := pdfRegex.FindAllStringSubmatch(htmlContent, -1)

	var pdfLinks []string
	for _, match := range matches {
		if len(match) > 1 {
			pdfLinks = append(pdfLinks, match[1]) // match[1] is the captured group (the URL)
		}
	}

	return pdfLinks
}

// Checks whether a given directory exists
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get info for the path
	if err != nil {
		return false // Return false if error occurs
	}
	return directory.IsDir() // Return true if it's a directory
}

// Creates a directory at given path with provided permissions
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Attempt to create directory
	if err != nil {
		log.Println(err) // Log error if creation fails
	}
}

// Verifies whether a string is a valid URL format
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri) // Try parsing the URL
	return err == nil                  // Return true if valid
}

// Removes duplicate strings from a slice
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool) // Map to track seen values
	var newReturnSlice []string    // Slice to store unique values
	for _, content := range slice {
		if !check[content] { // If not already seen
			check[content] = true                            // Mark as seen
			newReturnSlice = append(newReturnSlice, content) // Add to result
		}
	}
	return newReturnSlice
}

// hasDomain checks if the given string has a domain (host part)
func hasDomain(rawURL string) bool {
	// Try parsing the raw string as a URL
	parsed, err := url.Parse(rawURL)
	if err != nil { // If parsing fails, it's not a valid URL
		return false
	}
	// If the parsed URL has a non-empty Host, then it has a domain/host
	return parsed.Host != ""
}

// Extracts filename from full path (e.g. "/dir/file.pdf" → "file.pdf")
func getFilename(path string) string {
	return filepath.Base(path) // Use Base function to get file name only
}

// Removes all instances of a specific substring from input string
func removeSubstring(input string, toRemove string) string {
	result := strings.ReplaceAll(input, toRemove, "") // Replace substring with empty string
	return result
}

// Gets the file extension from a given file path
func getFileExtension(path string) string {
	return filepath.Ext(path) // Extract and return file extension
}

// Converts a raw URL into a sanitized PDF filename safe for filesystem
func urlToFilename(rawURL string) string {
	lower := strings.ToLower(rawURL) // Convert URL to lowercase
	lower = getFilename(lower)       // Extract filename from URL

	reNonAlnum := regexp.MustCompile(`[^a-z0-9]`)   // Regex to match non-alphanumeric characters
	safe := reNonAlnum.ReplaceAllString(lower, "_") // Replace non-alphanumeric with underscores

	safe = regexp.MustCompile(`_+`).ReplaceAllString(safe, "_") // Collapse multiple underscores into one
	safe = strings.Trim(safe, "_")                              // Trim leading and trailing underscores

	var invalidSubstrings = []string{
		"_pdf", // Substring to remove from filename
	}

	for _, invalidPre := range invalidSubstrings { // Remove unwanted substrings
		safe = removeSubstring(safe, invalidPre)
	}

	if getFileExtension(safe) != ".pdf" { // Ensure file ends with .pdf
		safe = safe + ".pdf"
	}

	return safe // Return sanitized filename
}

// Downloads a PDF from given URL and saves it in the specified directory
func downloadPDF(finalURL, outputDir string) bool {
	filename := strings.ToLower(urlToFilename(finalURL)) // Sanitize the filename
	filePath := filepath.Join(outputDir, filename)       // Construct full path for output file

	if fileExists(filePath) { // Skip if file already exists
		log.Printf("File already exists, skipping: %s", filePath)
		return false
	}

	client := &http.Client{Timeout: 15 * time.Minute} // Create HTTP client with timeout

	// Create a new request so we can set headers
	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		log.Printf("Failed to create request for %s: %v", finalURL, err)
		return false
	}

	// Set a User-Agent header
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to download %s: %v", finalURL, err)
		return false
	}
	defer resp.Body.Close() // Ensure response body is closed

	if resp.StatusCode != http.StatusOK { // Check if response is 200 OK
		log.Printf("Download failed for %s: %s", finalURL, resp.Status)
		return false
	}

	contentType := resp.Header.Get("Content-Type") // Get content type of response
	if !strings.Contains(contentType, "binary/octet-stream") &&
		!strings.Contains(contentType, "application/pdf") {
		log.Printf("Invalid content type for %s: %s (expected PDF)", finalURL, contentType)
		return false
	}

	var buf bytes.Buffer                     // Create a buffer to hold response data
	written, err := io.Copy(&buf, resp.Body) // Copy data into buffer
	if err != nil {
		log.Printf("Failed to read PDF data from %s: %v", finalURL, err)
		return false
	}
	if written == 0 { // Skip empty files
		log.Printf("Downloaded 0 bytes for %s; not creating file", finalURL)
		return false
	}

	out, err := os.Create(filePath) // Create output file
	if err != nil {
		log.Printf("Failed to create file for %s: %v", finalURL, err)
		return false
	}
	defer out.Close() // Ensure file is closed after writing

	if _, err := buf.WriteTo(out); err != nil { // Write buffer contents to file
		log.Printf("Failed to write PDF to file for %s: %v", finalURL, err)
		return false
	}

	log.Printf("Successfully downloaded %d bytes: %s → %s", written, finalURL, filePath) // Log success
	return true
}

// Performs HTTP GET request with a custom User-Agent and returns response body as string
func getDataFromURL(uri string) string {
	log.Println("Scraping", uri) // Log which URL is being scraped

	// Create a new HTTP client
	client := &http.Client{}

	// Create a new request
	request, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Println("Error creating request:", err)
		return ""
	}

	// Set a User-Agent header
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")

	// Send the request
	response, err := client.Do(request)
	if err != nil {
		log.Println("Request error:", err)
		return ""
	}
	defer func() {
		if cerr := response.Body.Close(); cerr != nil {
			log.Println("Error closing response body:", cerr)
		}
	}()

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Println("Error reading body:", err)
		return ""
	}

	return string(body)
}

// Append and write to file
func appendAndWriteToFile(path string, content string) {
	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	_, err = filePath.WriteString(content + "\n")
	if err != nil {
		log.Println(err)
	}
	err = filePath.Close()
	if err != nil {
		log.Println(err)
	}
}

// Read a file and return the contents
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Println(err)
	}
	return string(content)
}

// extractBaseDomain takes a URL string and returns only the bare domain name
// without any subdomains or suffixes (e.g., ".com", ".org", ".co.uk").
func extractBaseDomain(inputUrl string) string {
	// Parse the input string into a structured URL object
	parsedUrl, parseError := url.Parse(inputUrl)

	// If parsing fails, log the error and return an empty string
	if parseError != nil {
		log.Println("Error parsing URL:", parseError)
		return ""
	}

	// Extract the hostname (e.g., "sub.example.com")
	hostName := parsedUrl.Hostname()

	// Split the hostname into parts separated by "."
	// For example: "sub.example.com" -> ["sub", "example", "com"]
	parts := strings.Split(hostName, ".")

	// If there are at least 2 parts, the second last part is usually the domain
	// Example: "sub.example.com" -> "example"
	//          "blog.my-site.co.uk" -> "my-site"
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}

	// If splitting fails or domain structure is unusual, return the hostname itself
	return hostName
}

func main() {
	outputDir := "PDFs/" // Directory to store downloaded PDFs

	if !directoryExists(outputDir) { // Check if directory exists
		createDirectory(outputDir, 0o755) // Create directory with read-write-execute permissions
	}

	// The remote domain name.
	remoteDomainName := "https://blasterproducts.com"

	// The location to the local.
	localFile := extractBaseDomain(remoteDomainName) + ".html"
	// Check if the local file exists.
	if fileExists(localFile) {
		removeFile(localFile)
	}
	// The location to the remote url.
	remoteURL := []string{
		"https://blasterproducts.com/blaster_corporation_material_safety_data_sheets/",
		"https://blasterproducts.com/product/pb-blaster-penetrant/",
		"https://blasterproducts.com/product/de-icer/",
		"https://blasterproducts.com/product/hydraulic-jack-oil/",
		"https://blasterproducts.com/product/starting-fluid/",
		"https://blasterproducts.com/product/engine-degreaser/",
		"https://blasterproducts.com/product/surface-shield/",
		"https://blasterproducts.com/product/multi-max-lubricant/",
		"https://blasterproducts.com/product/air-tool-conditioner/",
		"https://blasterproducts.com/product/air-tool-lubricant/",
		"https://blasterproducts.com/product/brake-cleaner/",
		"https://blasterproducts.com/product/chain-and-cable-lubricant/",
		"https://blasterproducts.com/product/citrus-based-degreaser/",
		"https://blasterproducts.com/product/dry-lube/",
		"https://blasterproducts.com/product/garage-door-lubricant/",
		"https://blasterproducts.com/product/graphite-dry-lubricant/",
		"https://blasterproducts.com/product/heavy-duty-grease/",
		"https://blasterproducts.com/product/lock-dry-lubricant-de-icer/",
		"https://blasterproducts.com/product/multi-purpose-grease/",
		"https://blasterproducts.com/product/multi-purpose-lubricant/",
		"https://blasterproducts.com/product/battery-terminal-cleaner/",
		"https://blasterproducts.com/product/red-spray-grease/",
		"https://blasterproducts.com/product/fogging-oil/",
		"https://blasterproducts.com/product/rust-neutralizer/",
		"https://blasterproducts.com/product/multi-purpose-lubricant-2/",
		"https://blasterproducts.com/product/parts-washer-solvent/",
		"https://blasterproducts.com/product/penetrating-lithium-grease/",
		"https://blasterproducts.com/product/red-grease/",
		"https://blasterproducts.com/product/silicone-lubricant/",
		"https://blasterproducts.com/product/small-engine-tune-up/",
		"https://blasterproducts.com/product/white-lithium-grease/",
		"https://blasterproducts.com/product/bla608yfc/",
		"https://blasterproducts.com/product/bla610yfa/",
		"https://blasterproducts.com/product/bla611yf/",
		"https://blasterproducts.com/product/bla234c/",
		"https://blasterproducts.com/product/bla235d/",
		"https://blasterproducts.com/product/bla238a/",
		"https://blasterproducts.com/product/bla602yfd/",
		"https://blasterproducts.com/product/bla603yfd/",
		"https://blasterproducts.com/product/bla604yfc/",
		"https://blasterproducts.com/product/bla132d/",
		"https://blasterproducts.com/product/bla232d/",
		"https://blasterproducts.com/product/bla348a/",
		"https://blasterproducts.com/product/bla346a/",
		"https://blasterproducts.com/product/bla340a/",
		"https://blasterproducts.com/product/bla101h/",
		"https://blasterproducts.com/product/bla110a/",
		"https://blasterproducts.com/product/bla125a/",
		"https://blasterproducts.com/product/bla601yf/",
		"https://blasterproducts.com/product/bla607yfp/",
		"https://blasterproducts.com/product/bla301/",
		"https://blasterproducts.com/product/bla310/",
		"https://blasterproducts.com/product/bla107/",
		"https://blasterproducts.com/product/bla347p/",
		"https://blasterproducts.com/product/bla033/",
		"https://blasterproducts.com/product/bla606yf/",
		"https://blasterproducts.com/product/bla155/",
		"https://blasterproducts.com/product/bla151/",
		"https://blasterproducts.com/product/bla150/",
		"https://blasterproducts.com/product/bla153/",
		"https://blasterproducts.com/product/bla002/",
		"https://blasterproducts.com/product/bla007/",
		"https://blasterproducts.com/product/bla008/",
		"https://blasterproducts.com/product/bla017/",
		"https://blasterproducts.com/product/bla012/",
		"https://blasterproducts.com/product/bla022/",
		"https://blasterproducts.com/product/bla025/",
		"https://blasterproducts.com/product/bla505/",
		"https://blasterproducts.com/product/bla500/",
		"https://blasterproducts.com/product/bla501/",
		"https://blasterproducts.com/product/bla502/",
		"https://blasterproducts.com/product/bla503/",
		"https://blasterproducts.com/product/bla503ts/",
		"https://blasterproducts.com/product/power-chain-lubricant/",
		"https://blasterproducts.com/product/power-chain-lubricant-2/",
		"https://blasterproducts.com/product/air-filter-cleaner/",
		"https://blasterproducts.com/product/air-filter-oil/",
		"https://blasterproducts.com/product/mud-blaster/",
		"https://blasterproducts.com/product/factory-shine/",
	}
	// Loop over the urls and save content to file.
	for _, url := range remoteURL {
		// Call fetchPage to download the content of that page
		pageContent := getDataFromURL(url)
		// Append it and save it to the file.
		appendAndWriteToFile(localFile, pageContent)
	}
	// Read the file content
	fileContent := readAFileAsString(localFile)
	// Extract the URLs from the given content.
	extractedPDFURLs := extractPDFUrls(fileContent)
	// Remove duplicates from the slice.
	extractedPDFURLs = removeDuplicatesFromSlice(extractedPDFURLs)
	// Loop through all extracted PDF URLs
	for _, urls := range extractedPDFURLs {
		if !hasDomain(urls) {
			urls = remoteDomainName + urls

		}
		if isUrlValid(urls) { // Check if the final URL is valid
			downloadPDF(urls, outputDir) // Download the PDF
		}
	}
}

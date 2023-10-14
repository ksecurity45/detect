package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"
	"bufio"
	"os"
	"net/url"
	"regexp"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// ResponseData represents the data to be captured and serialized.
type ResponseData struct {
	URL          string   `json:"url"`
	ResponseBody string   `json:"response_body"`
	Network      []string `json:"network"`
	Timestamp    string   `json:"timestamp"`
}

func main() {
    var url, outputFilePath string

    flag.StringVar(&url, "u", "", "URL to visit")
    flag.StringVar(&outputFilePath, "o", "./detect_result.json", "Output file path")
    flag.Parse()

    if url != "" {
        // Process the URL provided via the -u flag and save the result to the specified output file
        processURL(url, outputFilePath)
    } else {
        // Read URLs from stdin and filter valid and resolved domains
        urls := readAndFilterURLs()
        for _, u := range urls {
            // Process the URL and save the result to the specified output file
            processURL(u, outputFilePath)
        }
    }
}


// processURL processes a single URL and appends the JSON data to a file.
func processURL(url string, outputFilePath string) {
    fmt.Println("Get:", url)
    ctx, cancel := createContextWithTimeout(10 * time.Second)
    responseData, err := captureWebData(ctx, url)
    cancel()
    if err != nil {
        log.Printf("Failed to capture web data for %s: %v", url, err)
        return
    }
    outputJSON(responseData, outputFilePath)
}

// createContextWithTimeout creates a context with a timeout.
func createContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancelTimeout := context.WithTimeout(context.Background(), timeout)
	return ctx, cancelTimeout
}

// captureWebData captures web data for a given URL.
func captureWebData(ctx context.Context, url string) (ResponseData, error) {
	// Initialize a context for network event handling
	ctx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	var responseBody string
	var requestURLs []string

	// Listen for network events and extract request URLs
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ev, ok := ev.(*network.EventRequestWillBeSent); ok {
			// Check if the URL doesn't start with "data:" and isn't already in the list
			if !strings.HasPrefix(ev.Request.URL, "data:") && !contains(requestURLs, ev.Request.URL) {
				requestURLs = append(requestURLs, ev.Request.URL)
			}
		}
	})

	// Navigate to the URL and wait for it to load
	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
	); err != nil {
		return ResponseData{}, err
	}

	script := `document.documentElement.outerHTML;` // JavaScript code to get the entire HTML content
	if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(script, &responseBody)); err != nil {
		return ResponseData{}, err
	}

	responseBody = RemoveDataURIs(responseBody)
	maxBodyLength := 50 * 1024
	if len(responseBody) > maxBodyLength {
		responseBody = responseBody[:maxBodyLength]
	}

	responseData := ResponseData{
		URL:          url,
		ResponseBody: responseBody,
		Network:      requestURLs,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
	}

	return responseData, nil
}


// outputJSON serializes the ResponseData struct to JSON and appends it to a file.
func outputJSON(data ResponseData, filePath string) {
    file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
    if err != nil {
        log.Printf("Failed to open file: %v", err)
        return
    }
    defer file.Close()

    encoder := json.NewEncoder(file)
    if err := encoder.Encode(data); err != nil {
        log.Printf("Failed to write JSON data to file: %v", err)
    }
}

// contains checks if a URL is in a slice.
func contains(slice []string, url string) bool {
	for _, s := range slice {
		if s == url {
			return true
		}
	}
	return false
}

func RemoveDataURIs(htmlBody string) string {
    // Define a regular expression to match data:// URIs
    dataURIPattern := `data:[^;]+;[^,]+,`

    // Use the regular expression to find and replace data:// URIs with an empty string
    replacedHTMLBody := regexp.MustCompile(dataURIPattern).ReplaceAllString(htmlBody, "")

    return replacedHTMLBody
}

// readAndFilterURLs reads URLs from stdin and filters valid and resolved domains
func readAndFilterURLs() []string {
	var validURLs []string
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			url := scanner.Text()
			// print url
			
			if isValidURL(url) {
				validURLs = append(validURLs, url)
			}
		}
	}

	return validURLs
}

// isValidURL checks if a URL is valid and resolved
func isValidURL(urll string) bool {
    u, err := url.Parse(urll)
    if err != nil {
        return false // Invalid URL format
    }

    if u.Scheme != "http" && u.Scheme != "https" {
        return false // URL must have an "http" or "https" scheme
    }

    // You can check other conditions, such as ensuring the URL is well-formed and follows other criteria if needed.

    return true // If the URL meets the specified criteria
}
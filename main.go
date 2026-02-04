package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"net/url"
	"os"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/briandowns/spinner"
	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	"github.com/ttacon/chalk"
	"github.com/valyala/fasthttp"
)

type resp struct {
	status  int
	latency int64
	size    int
}

func main() {
	uri := flag.String("url", "", "url yang akan di benchmark. (Required)")
	clients := flag.Int("c", 10, "jumlah koneksi yang digunakan untuk benchmark.")
	pipeliningFactor := flag.Int("p", 1, "jumlah pipelining yang digunakan untuk benchmark.")
	runtime := flag.Int("d", 10, "durasi benchmark dalam detik.")
	timeout := flag.Int("t", 10, "jumlah detik untuk timeout koneksi.")
	debug := flag.Bool("debug", false, "untuk debug koneksi.")
	flag.Parse()

	if *uri == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	parsedURL, err := url.Parse(*uri)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		fmt.Printf("Invalid URI: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("running %vs test @ %v\n", *runtime, *uri)
	totalWorkers := *clients * *pipeliningFactor
	fmt.Printf("%v concurrent workers (connections × pipelining).\n", totalWorkers)

	// Channels dan metrik
	respChan := make(chan *resp, 1000)
	errChan := make(chan error, 1000)
	done := make(chan struct{})

	latencies := hdrhistogram.New(1, 10000, 5)
	requests := hdrhistogram.New(1, 1000000, 5)
	throughput := hdrhistogram.New(1, 100000000000, 5)

	var bytesTransferred int64
	var totalBytes int64
	var respCounter int64
	var totalResp int64
	resp2xx := 0
	respN2xx := 0
	errors := 0
	timeouts := 0

	// Jalankan workers
	ctx, cancel := context.WithCancel(context.Background())
	go runWorkers(ctx, totalWorkers, time.Duration(*timeout)*time.Second, *uri, parsedURL, respChan, errChan, *debug)

	// Ticker dan timeout
	ticker := time.NewTicker(time.Second)
	runTimer := time.NewTimer(time.Duration(*runtime) * time.Second)
	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spin.Suffix = " Running benchmark..."
	spin.Start()

	for {
		select {
		case err := <-errChan:
			errors++
			if *debug {
				fmt.Printf("error: %s\n", err.Error())
			}
			if err == fasthttp.ErrTimeout {
				timeouts++
			}
		case res := <-respChan:
			bytesTransferred += int64(res.size)
			totalBytes += int64(res.size)
			respCounter++
			totalResp++
			if res.status >= 200 && res.status < 300 {
				latencies.RecordValue(res.latency)
				resp2xx++
			} else {
				respN2xx++
			}
		case <-ticker.C:
			requests.RecordValue(respCounter)
			respCounter = 0
			throughput.RecordValue(bytesTransferred)
			bytesTransferred = 0
		case <-runTimer.C:
			spin.Stop()
			cancel()    // Beritahu semua worker untuk berhenti
			close(done) // Optional: untuk cleanup
			goto printResults
		}
	}

printResults:
	// Tunggu sebentar agar response terakhir masuk (drain channel)
	time.Sleep(500 * time.Millisecond)

	// ... (bagian print hasil tetap sama, hanya sedikit cleanup format)
	// Latency table
	shortLatency := tablewriter.NewWriter(os.Stdout)
	shortLatency.SetHeader([]string{"Stat", "2.5%", "50%", "97.5%", "99%", "Avg", "Stdev", "Max"})
	// ... (header color sama)
	shortLatency.Append([]string{
		chalk.Bold.TextStyle("Latency"),
		fmt.Sprintf("%v ms", latencies.ValueAtPercentile(2.5)),
		fmt.Sprintf("%v ms", latencies.ValueAtPercentile(50)),
		fmt.Sprintf("%v ms", latencies.ValueAtPercentile(97.5)),
		fmt.Sprintf("%v ms", latencies.ValueAtPercentile(99)),
		fmt.Sprintf("%.2f ms", latencies.Mean()),
		fmt.Sprintf("%.2f ms", latencies.StdDev()),
		fmt.Sprintf("%v ms", latencies.Max()),
	})
	shortLatency.Render()

	// Requests & throughput table
	requestsTable := tablewriter.NewWriter(os.Stdout)
	requestsTable.SetHeader([]string{"Stat", "1%", "2.5%", "50%", "97.5%", "Avg", "Stdev", "Min"})
	requestsTable.Append([]string{
		chalk.Bold.TextStyle("Req/Sec"),
		fmt.Sprintf("%v", requests.ValueAtPercentile(1)),
		fmt.Sprintf("%v", requests.ValueAtPercentile(2.5)),
		fmt.Sprintf("%v", requests.ValueAtPercentile(50)),
		fmt.Sprintf("%v", requests.ValueAtPercentile(97.5)),
		fmt.Sprintf("%.2f", requests.Mean()),
		fmt.Sprintf("%.2f ms", requests.StdDev()),
		fmt.Sprintf("%v", requests.Min()),
	})
	requestsTable.Append([]string{
		chalk.Bold.TextStyle("Bytes/Sec"),
		humanize.Bytes(uint64(throughput.ValueAtPercentile(1))),
		humanize.Bytes(uint64(throughput.ValueAtPercentile(2.5))),
		humanize.Bytes(uint64(throughput.ValueAtPercentile(50))),
		humanize.Bytes(uint64(throughput.ValueAtPercentile(97.5))),
		humanize.Bytes(uint64(throughput.Mean())),
		humanize.Bytes(uint64(throughput.StdDev())),
		humanize.Bytes(uint64(throughput.Min())),
	})
	requestsTable.Render()

	fmt.Printf("\n%v 2xx responses, %v non 2xx responses.\n", resp2xx, respN2xx)
	fmt.Printf("%v total requests in %v seconds, %s read.\n",
		formatBigNum(float64(totalResp)), *runtime, humanize.Bytes(uint64(totalBytes)))
	if errors > 0 {
		fmt.Printf("%v total errors (%v timeouts).\n",
			formatBigNum(float64(errors)), formatBigNum(float64(timeouts)))
	}
	fmt.Println("Done!")
}

func formatBigNum(i float64) string {
	if i < 1000 {
		return fmt.Sprintf("%.0f", i)
	}
	return fmt.Sprintf("%.0fk", math.Round(i/1000))
}

// runWorkers menjalankan worker goroutine
func runWorkers(ctx context.Context, workers int, timeout time.Duration, uri string, u *url.URL, respChan chan<- *resp, errChan chan<- error, debug bool) {
	client := &fasthttp.Client{
		MaxConnsPerHost:     workers, // Batasi total connection
		ReadTimeout:         timeout,
		WriteTimeout:        timeout,
		MaxIdleConnDuration: 30 * time.Second,
	}

	for i := 0; i < workers; i++ {
		go func() {
			req := fasthttp.AcquireRequest()
			res := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseRequest(req)
			defer fasthttp.ReleaseResponse(res)

			req.SetRequestURI(uri)
			req.Header.SetMethod(fasthttp.MethodPost)
			req.SetBody([]byte("hello, world!"))

			for {
				select {
				case <-ctx.Done():
					return
				default:
					start := time.Now()
					if err := client.DoTimeout(req, res, timeout); err != nil {
						errChan <- err
					} else {
						// Hitung size lebih akurat: body + headers
						size := len(res.Body()) + res.Header.Len()

						respChan <- &resp{
							status:  res.StatusCode(),
							latency: time.Since(start).Milliseconds(),
							size:    size,
						}
						res.ResetBody() // Keep headers jika mau reuse connection
					}
				}
			}
		}()
	}
}

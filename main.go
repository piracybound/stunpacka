package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	KEY1        = 0xFFFEA4C8
	SIZE_OFFSET = 0x78752C
	HEADER      = "-- manifest & lua provided by: https://www.piracybound.com/discord\n-- via manilua\n"
	
	NAME    = "stunpacka"
	WEBSITE = "https://www.piracybound.com/discord"
	CREDITS = "by @uchks on Discord & Telegram"
)

type Config struct {
	OutputDir      string
	Verbose        bool
	MaxConcurrency int
	ForceOverwrite bool
}

type FileProcessor struct {
	config Config
	stats  struct {
		processed int
		failed    int
		skipped   int
		mutex     sync.Mutex
	}
}

type ProcessResult struct {
	Filename string
	Success  bool
	Message  string
	Error    error
}

func NewFileProcessor(config Config) *FileProcessor {
	return &FileProcessor{
		config: config,
	}
}

func (fp *FileProcessor) ProcessFile(inputPath string) ProcessResult {
	result := ProcessResult{
		Filename: inputPath,
	}

	if !strings.HasSuffix(inputPath, ".st") {
		result.Message = "Not a .st file"
		result.Success = false
		return result
	}

	outputPath := strings.TrimSuffix(inputPath, ".st") + ".lua"
	if fp.config.OutputDir != "" {
		outputPath = filepath.Join(fp.config.OutputDir, filepath.Base(outputPath))
	}

	if _, err := os.Stat(outputPath); err == nil && !fp.config.ForceOverwrite {
		result.Message = "output file already exists (use -force to overwrite)"
		result.Success = false
		return result
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to read file: %w", err)
		result.Success = false
		return result
	}

	if len(data) < 12 {
		result.Error = fmt.Errorf("file too small (minimum 12 bytes required)")
		result.Success = false
		return result
	}

	encTime := binary.LittleEndian.Uint32(data[0:4])
	compSize := binary.LittleEndian.Uint32(data[4:8])
	encDcSize := binary.LittleEndian.Uint32(data[8:12])

	time := encTime ^ KEY1
	decompressedSize := ((encDcSize ^ KEY1) + SIZE_OFFSET) & 0xffffffff
	key := byte(time & 0xff)

	compData := data[12:]
	if uint32(len(compData)) != compSize {
		result.Error = fmt.Errorf("compressed data size mismatch (expected %d, got %d)", compSize, len(compData))
		result.Success = false
		return result
	}

	decryptedData := make([]byte, len(compData))
	for i := range compData {
		decryptedData[i] = compData[i] ^ key
	}

	reader := bytes.NewReader(decryptedData)
	zlibReader, err := zlib.NewReader(reader)
	if err != nil {
		result.Error = fmt.Errorf("failed to create zlib reader: %w", err)
		result.Success = false
		return result
	}

	decompressed := bytes.NewBuffer(make([]byte, 0, decompressedSize))
	
	_, err = io.Copy(decompressed, zlibReader)
	zlibReader.Close()
	
	if err != nil {
		result.Error = fmt.Errorf("failed to decompress data: %w", err)
		result.Success = false
		return result
	}

	decompressedData := decompressed.Bytes()
	if len(decompressedData) < 12 {
		result.Error = fmt.Errorf("decompressed data too small")
		result.Success = false
		return result
	}

	if fp.config.OutputDir != "" {
		if err = os.MkdirAll(fp.config.OutputDir, 0755); err != nil {
			result.Error = fmt.Errorf("failed to create output directory: %w", err)
			result.Success = false
			return result
		}
	}

	outputData := make([]byte, len(HEADER)+len(decompressedData)-512)
	copy(outputData, HEADER)
	copy(outputData[len(HEADER):], decompressedData[512:])
	
	if err = os.WriteFile(outputPath, outputData, 0644); err != nil {
		result.Error = fmt.Errorf("failed to write output file: %w", err)
		result.Success = false
		return result
	}

	result.Message = fmt.Sprintf("converted to %s", outputPath)
	result.Success = true
	return result
}

func (fp *FileProcessor) ProcessFiles(files []string) {
	if len(files) == 0 {
		fmt.Println("no files to process")
		return
	}

	results := make(chan ProcessResult, len(files))
	
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, fp.config.MaxConcurrency)
	
	startTime := time.Now()
	fmt.Printf("processing %d files with %d workers...\n", len(files), fp.config.MaxConcurrency)
	
	for _, file := range files {
		wg.Add(1)
		semaphore <- struct{}{}
		
		go func(f string) {
			defer wg.Done()
			defer func() { <-semaphore }()
			
			result := fp.ProcessFile(f)
			results <- result
			
			fp.stats.mutex.Lock()
			if result.Success {
				fp.stats.processed++
			} else if strings.HasPrefix(result.Message, "not a .st file") {
				fp.stats.skipped++
			} else {
				fp.stats.failed++
			}
			fp.stats.mutex.Unlock()
		}(file)
	}
	
	go func() {
		wg.Wait()
		close(results)
	}()
	
	for result := range results {
		if result.Success {
			fmt.Printf("✓ %s: %s\n", filepath.Base(result.Filename), result.Message)
		} else {
			if result.Error != nil {
				fmt.Printf("✗ %s: %s\n", filepath.Base(result.Filename), result.Error)
			} else {
				fmt.Printf("- %s: %s\n", filepath.Base(result.Filename), result.Message)
			}
		}
		
		if fp.config.Verbose && result.Error != nil {
			fmt.Printf("  details: %v\n", result.Error)
		}
	}
	
	duration := time.Since(startTime)
	fmt.Printf("\nsummary:\n")
	fmt.Printf("  processed: %d\n", fp.stats.processed)
	fmt.Printf("  failed: %d\n", fp.stats.failed)
	fmt.Printf("  skipped: %d\n", fp.stats.skipped)
	fmt.Printf("  time: %.2f seconds\n", duration.Seconds())
}

func ShowCredits() {
	fmt.Println("======================================")
	fmt.Printf("%s\n", NAME)
	fmt.Println("--------------------------------------")
	fmt.Println(CREDITS)
	fmt.Printf("%s\n", WEBSITE)
	fmt.Println("======================================")
}

func InteractiveMode(fp *FileProcessor) {
	fmt.Printf("====== %s ======\n", NAME)
	fmt.Println("drag and drop .st files onto this window, or type filenames")
	fmt.Println("commands:")
	fmt.Println("  help     - show this dialogue")
	fmt.Println("  credits  - show credits")
	fmt.Println("  website  - open discord")
	fmt.Println("  exit     - exit stunpacka")
	fmt.Println("======================================")

	var files []string
	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		
		switch input {
		case "exit", "quit":
			return
		case "help":
			fmt.Println("drag and drop .st files onto this window, or type filenames")
			fmt.Println("commands:")
			fmt.Println("  help     - show this dialogue")
			fmt.Println("  credits  - show credits")
			fmt.Println("  website  - open discord")
			fmt.Println("  exit     - exit stunpacka")
			continue
		case "clear", "cls":
			fmt.Print("\033[H\033[2J")
			continue
		case "credits", "about":
			ShowCredits()
			continue
		case "discord":
			var cmd *exec.Cmd
			switch runtime.GOOS {
			case "windows":
				cmd = exec.Command("cmd", "/c", "start", WEBSITE)
			case "darwin":
				cmd = exec.Command("open", WEBSITE)
			default:
				cmd = exec.Command("xdg-open", WEBSITE)
			}
			
			if err := cmd.Start(); err != nil {
				fmt.Printf("failed to open the discord server: %v\n", err)
				fmt.Printf("%s\n", WEBSITE)
			} else {
			}
			continue
		}
		
		inputFiles := parseInput(input)
		if len(inputFiles) > 0 {
			files = append(files, inputFiles...)
			fp.ProcessFiles(inputFiles)
		}
	}
}

func parseInput(input string) []string {
	var result []string
	var current string
	inQuotes := false
	
	for _, r := range input {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ' ':
			if !inQuotes {
				if current != "" {
					result = append(result, current)
					current = ""
				}
			} else {
				current += string(r)
			}
		default:
			current += string(r)
		}
	}
	
	if current != "" {
		result = append(result, current)
	}
	
	for i, path := range result {
		result[i] = strings.Trim(path, "\"")
	}
	
	return result
}

func main() {
	outputDir := flag.String("out", "", "output directory for converted files")
	verbose := flag.Bool("verbose", false, "enable verbose output")
	maxConcurrency := flag.Int("workers", runtime.NumCPU(), "maximum number of concurrent workers")
	forceOverwrite := flag.Bool("force", false, "force overwrite existing files")
	help := flag.Bool("help", false, "show help")
	
	flag.Parse()
	
	if *help {
		fmt.Println("usage: stunpacka.exe [options] [files...]")
		flag.PrintDefaults()
		return
	}
	
	config := Config{
		OutputDir:      *outputDir,
		Verbose:        *verbose,
		MaxConcurrency: *maxConcurrency,
		ForceOverwrite: *forceOverwrite,
	}
	
	fp := NewFileProcessor(config)
	
	if flag.NArg() > 0 {
		fp.ProcessFiles(flag.Args())
	} else {
		InteractiveMode(fp)
	}
}
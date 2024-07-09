/*
GetSTRM.go

Copyright (c) 2024 Jules Potvin

This software is licensed under the Creative Commons Attribution-NonCommercial 4.0 International License.
You may use, distribute, and modify this code under the terms of the license.

To view a copy of this license, visit http://creativecommons.org/licenses/by-nc/4.0/

This code is provided "as is", without warranty of any kind, express or implied, including but not limited to the
warranties of merchantability, fitness for a particular purpose and noninfringement. In no event shall the
authors or copyright holders be liable for any claim, damages or other liability, whether in an action of contract,
tort or otherwise, arising from, out of or in connection with the software or the use or other dealings in the software.
*/

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const version = "1.0.2"

type Stream struct {
	URL        string `json:"url"`
	TvgName    string `json:"tvg_name"`
	GroupTitle string `json:"group_title"`
}

type Config struct {
	Name           string   `json:"name"`
	LogLevel       int      `json:"logLevel"`
	TvShowsDir     string   `json:"tvShowsDir"`
	MoviesDir      string   `json:"moviesDir"`
	JsonURLs       []string `json:"jsonURLs"`
	M3UURLs        []string `json:"m3uURLs"`
	LogFile        string   `json:"logFile"`
	FileType       string   `json:"fileType"`
	WorkingDir     string   `json:"workingDir"`
	LogDir         string   `json:"logDir"`
	RetainDownload int      `json:"retainDownload"`
	DownloadDir    string   `json:"downloadDir"`
	LimitDelete    int      `json:"limitDelete"`
	UseGroup       int      `json:"useGroup"`
	DefaultGroup   string   `json:"defaultGroup"`
	ExcludeGroup   string   `json:"excludeGroup"`
	IncludeGroup   string   `json:"includeGroup"`
}

var (
	name             string
	logLevel         int
	tvShowsDir       string
	moviesDir        string
	jsonURLs         []string
	m3uURLs          []string
	logFile          string
	fileTypes        []string
	workingDir       string
	logDir           string
	createdDirs      int
	keptStrmFiles    int
	removedStrmFiles int
	removedEmptyDirs int
	logFileHandle    *os.File
	downloadDir      string
	useGroup         int
	defaultGroup     string
	excludeGroups    []string
	includeGroups    []string
	keepFiles        map[string]bool // Declare keepFiles here
)

func main() {
	// Command line arguments
	configFile := flag.String("config", "", "Path to the configuration file")
	nameFlag := flag.String("name", "", "Name to be printed in the log file and used for config file creation")
	logLevelFlag := flag.Int("logLevel", -1, "Log level: 0 = silent, 1 = basic information, 3 = all debug and errors")
	tvShowsDirFlag := flag.String("tvShowsDir", "", "Directory for TV shows")
	moviesDirFlag := flag.String("moviesDir", "", "Directory for movies")
	jsonURLFlag := flag.String("jsonURL", "", "URL of the JSON file")
	m3uURLFlag := flag.String("m3u", "", "URL of the M3U file")
	logFileFlag := flag.String("logFile", "", "Name of the log file")
	fileTypeFlag := flag.String("fileType", "avi,flv,m4v,mkv,mkv2,mkv5,mkvv,mp4,mp41,mp42,mp44,mpg,wmv", "Comma separated list of valid strm file types")
	workingDirFlag := flag.String("workingDir", "", "Working directory")
	logDirFlag := flag.String("logDir", "", "Directory for log files")
	helpFlag := flag.Bool("help", false, "Show help message")
	retainDownloadFlag := flag.Int("retainDownload", 0, "Set to 1 to keep downloaded files, 0 to delete (default: 0)")
	downloadDirFlag := flag.String("downloadDir", "", "Directory to keep downloaded files (overrides default)")
	limitDeleteFlag := flag.Int("limitDelete", 25, "Maximum number of .strm files to delete (default: 25)")
	useGroupFlag := flag.Int("useGroup", 0, "Set to 1 to use group title in directory structure, 0 to not use (default: 0)")
	defaultGroupFlag := flag.String("defaultGroup", "Dummy", "Default group title if useGroup is set and group is not specified (default: Dummy)")

	excludeGroupFlag := flag.String("excludeGroup", "", "Comma separated list of groups to exclude")
	includeGroupFlag := flag.String("includeGroup", "", "Comma separated list of groups to include")

	versionFlag := flag.Bool("version", false, "Display the version information")

	stats := map[string]int{
		"createdDirs":       0,
		"keptStrmFiles":     0,
		"removedStrmFiles":  0,
		"removedEmptyDirs":  0,
		"processedJsonURLs": 0,
		"processedM3UURLs":  0,
		"rejectedFileExts":  0,
	}

	flag.Parse()

	if *versionFlag {
		fmt.Printf("GetSTRM version %s\n", version)
		// Print copyright notice
		fmt.Println("Copyright (c) 2024 Jules Potvin")
		fmt.Println("Licensed under the Creative Commons Attribution-NonCommercial 4.0 International License")
		fmt.Println("http://creativecommons.org/licenses/by-nc/4.0/")
		fmt.Println()
		return
	}

	if *helpFlag {
		showHelp()
		return
	}

	// Load configuration from file if provided
	var config *Config
	var err error
	if *configFile != "" {
		config, err = loadConfig(*configFile)
		if err != nil {
			fmt.Println("Error loading config file:", err)
			return
		}
	} else {
		config = &Config{}
	}

	// Override config with command line parameters if provided
	if *nameFlag != "" {
		config.Name = *nameFlag
	}
	if *logLevelFlag != -1 {
		config.LogLevel = *logLevelFlag
	}
	if *tvShowsDirFlag != "" {
		config.TvShowsDir = *tvShowsDirFlag
	}
	if *moviesDirFlag != "" {
		config.MoviesDir = *moviesDirFlag
	}
	if *jsonURLFlag != "" {
		config.JsonURLs = append(config.JsonURLs, *jsonURLFlag)
	}
	if *m3uURLFlag != "" {
		config.M3UURLs = append(config.M3UURLs, *m3uURLFlag)
	}
	if *logFileFlag != "" {
		config.LogFile = *logFileFlag
		if strings.ContainsAny(config.LogFile, `/\`) {
			fmt.Println("Error: logFile should be a file name only, not a path.")
			return
		}
	}
	if *fileTypeFlag != "" {
		config.FileType = *fileTypeFlag
	}
	if *workingDirFlag != "" {
		config.WorkingDir = *workingDirFlag
	} else {
		workingDir, _ = os.Getwd() // Default to current working directory
	}
	if *logDirFlag != "" {
		config.LogDir = *logDirFlag
	}
	if *retainDownloadFlag != 0 {
		config.RetainDownload = *retainDownloadFlag
	}
	if *downloadDirFlag != "" {
		config.DownloadDir = *downloadDirFlag
	}
	if *limitDeleteFlag != 25 {
		config.LimitDelete = *limitDeleteFlag
	}
	if *useGroupFlag != 0 {
		config.UseGroup = *useGroupFlag
	}
	if *defaultGroupFlag != "Dummy" {
		config.DefaultGroup = *defaultGroupFlag
	}
	if *excludeGroupFlag != "" {
		config.ExcludeGroup = *excludeGroupFlag
	}
	if *includeGroupFlag != "" {
		config.IncludeGroup = *includeGroupFlag
	}

	// Ensure all required parameters are set
	missingParams := []string{}
	if config.TvShowsDir == "" {
		missingParams = append(missingParams, "tvShowsDir")
	}
	if config.MoviesDir == "" {
		missingParams = append(missingParams, "moviesDir")
	}
	if len(config.JsonURLs) == 0 && len(config.M3UURLs) == 0 {
		missingParams = append(missingParams, "jsonURL or m3u")
	}
	if config.WorkingDir != "" {
		workingDir = config.WorkingDir
	}

	if len(missingParams) > 0 {
		fmt.Printf("Error: Missing required parameters: %s\n", strings.Join(missingParams, ", "))
		showHelp()
		return
	}

	// Validate working directory
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		fmt.Printf("Error: Working directory does not exist: %s\n", workingDir)
		return
	}

	// Set logDir if not provided
	if config.LogDir == "" {
		config.LogDir = filepath.Join(workingDir, "Log")
	}

	// Create log directory if it does not exist
	if _, err := os.Stat(config.LogDir); os.IsNotExist(err) {
		os.MkdirAll(config.LogDir, os.ModePerm)
		fmt.Println("Created log directory:", config.LogDir)
	}

	// Set downloadDir and downloadDir
	if config.DownloadDir != "" {
		downloadDir = config.DownloadDir
	} else {
		downloadDir = filepath.Join(workingDir, "Download")
		if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
			os.MkdirAll(downloadDir, os.ModePerm)
			fmt.Println("Created directory:", downloadDir)
		}
	}

	// Set configuration variables
	name = config.Name
	logLevel = config.LogLevel
	tvShowsDir = config.TvShowsDir
	moviesDir = config.MoviesDir
	jsonURLs = config.JsonURLs
	m3uURLs = config.M3UURLs
	fileTypes = strings.Split(config.FileType, ",")
	logDir = config.LogDir
	useGroup = config.UseGroup
	defaultGroup = config.DefaultGroup
	// Ensure proper trimming, splitting, and converting to lowercase of excludeGroup and includeGroup
	excludeGroups = filterEmptyStrings(strings.Split(strings.ToLower(strings.TrimSpace(config.ExcludeGroup)), ","))
	includeGroups = filterEmptyStrings(strings.Split(strings.ToLower(strings.TrimSpace(config.IncludeGroup)), ","))

	// Validate that includeGroup and excludeGroup do not overlap
	if hasCommonElement(excludeGroups, includeGroups) {
		fmt.Println("Error: includeGroup and excludeGroup cannot contain the same group names.")
		return
	}

	if config.LogFile != "" {
		logFile = filepath.Join(config.LogDir, config.LogFile)
	} else {
		logFile = ""
	}

	// Initialize keepFiles map
	keepFiles = make(map[string]bool)

	// Open log file for appending if provided
	if config.LogFile != "" {
		logFileHandle, err = os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Println("Error opening log file:", err)
			return
		}
		defer logFileHandle.Close()
	}

	// Log the start of the script
	if name != "" {
		logMessage(fmt.Sprintf("Starting GetSTRM: %s", name))
	} else {
		logMessage("Starting GetSTRM")
	}
	logDebug("Level 3 Debug turned on")
	logMessage("Copyright (c) 2024 Jules Potvin. Licensed under CC BY-NC 4.0")
	var streams []Stream

	// Process JSON inputs
	for i, jsonURL := range jsonURLs {
		logMessage(fmt.Sprintf("Processing JSON URL: %s", jsonURL))
		jsonStreams, err := processJSON(jsonURL, i)
		if err != nil {
			logError("Error processing JSON file:", err)
			os.Exit(1) // Exit the script on error
		}
		streams = append(streams, jsonStreams...)
	}

	// Process M3U inputs
	for i, m3uURL := range m3uURLs {
		logMessage(fmt.Sprintf("Processing M3U URL: %s", m3uURL))
		m3uStreams, err := processM3U(m3uURL, i, stats)
		if err != nil {
			logError("Error processing M3U file:", err)
			os.Exit(1) // Exit the script on error
		}
		streams = append(streams, m3uStreams...)
	}

	// Pass keepFiles to processStreams
	processStreams(streams, stats, keepFiles)

	// Clean up empty directories
	removeEmptyDirs(tvShowsDir, stats, config.LimitDelete)
	removeEmptyDirs(moviesDir, stats, config.LimitDelete)

	// Print the statistics for each URL
	printStatistics(stats)

	// Write keepFiles to disk if log level is 3
	if logLevel == 1 {
		err := writeKeepFilesToDisk()
		if err != nil {
			logError("Error writing keepFiles to disk:", err)
		}
	}

	// Remove downloaded files if retainDownload is 0
	if config.RetainDownload == 0 {
		removeDownloadedFiles()
	}

	// Log the end of the script
	logMessage(fmt.Sprintf("End GETVOD with URL %s", jsonURLs))

	// Save config if it wasn't loaded from a file
	if *configFile == "" && *nameFlag != "" {
		err := saveConfigToFile(config)
		if err != nil {
			fmt.Println("Error saving config to file:", err)
		} else {
			fmt.Printf("Configuration saved to %s.json\n", name)
		}
	}
}

func writeKeepFilesToDisk() error {
	filePath := filepath.Join(logDir, "keepFiles.txt")
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	for path := range keepFiles {
		_, err := file.WriteString(path + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}

func removeDownloadedFiles() {
	files, err := ioutil.ReadDir(downloadDir)
	if err != nil {
		logError("Error reading download directory:", err)
		return
	}

	for _, file := range files {
		filePath := filepath.Join(downloadDir, file.Name())
		if !file.IsDir() {
			logMessage(fmt.Sprintf("Removing downloaded file: %s", filePath))
			if err := os.Remove(filePath); err != nil {
				logError("Error removing file:", err)
			}
		}
	}
}

func processStreams(streams []Stream, stats map[string]int, keepFiles map[string]bool) map[string]int {
	// Regex to identify TV shows
	tvShowRegex := regexp.MustCompile(`S\d{1,2}E\d{1,3}`)
	logDebug("Process Streams Start")
	// Create root directories
	os.MkdirAll(tvShowsDir, os.ModePerm)
	os.MkdirAll(moviesDir, os.ModePerm)

	// Log excludeGroups and includeGroups for debugging
	logMessage(fmt.Sprintf("Exclude Groups: %v", excludeGroups))
	logMessage(fmt.Sprintf("Include Groups: %v", includeGroups))

	for _, stream := range streams {
		groupTitle := strings.ToLower(strings.TrimSpace(stream.GroupTitle))
		if groupTitle == "" {
			groupTitle = defaultGroup
		}

		// Log the current group being processed
		logDebug(fmt.Sprintf("Processing group: %s", groupTitle))

		// Skip excluded groups
		if contains(excludeGroups, groupTitle) {
			logDebug(fmt.Sprintf("Excluding group: %s", groupTitle))
			continue
		}

		// Include only specified groups
		if len(includeGroups) > 0 && !contains(includeGroups, groupTitle) {
			logDebug(fmt.Sprintf("Not in include group: %s", groupTitle))
			continue
		}

		if tvShowRegex.MatchString(stream.TvgName) {
			// It's a TV show
			processTVShow(stream, tvShowsDir, groupTitle, tvShowRegex, keepFiles, stats)
		} else {
			// It's a movie
			processMovie(stream, moviesDir, groupTitle, keepFiles, stats)
		}
	}

	return stats
}

func contains(slice []string, item string) bool {
	item = strings.ToLower(strings.TrimSpace(item))
	for _, s := range slice {
		logDebug(fmt.Sprintf("Checking if %s equals %s", strings.TrimSpace(s), item))
		if strings.EqualFold(strings.TrimSpace(s), item) {
			return true
		}
	}
	return false
}

func removeEmptyDirs(rootDir string, stats map[string]int, limit int) {
	// Create a case-insensitive map for keepFiles
	ciKeepFiles := make(map[string]bool)
	for k := range keepFiles {
		ciKeepFiles[strings.ToLower(k)] = true
	}

	deletions := 0

	filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logError("Error accessing path", path, ":", err)
			return err
		}
		if d.IsDir() {
			// Remove files that are not in the ciKeepFiles map
			files, err := ioutil.ReadDir(path)
			if err != nil {
				logError("Error reading directory:", err)
				return err
			}
			for _, file := range files {
				if deletions >= limit {
					logMessage("Deletion limit reached.")
					return filepath.SkipDir
				}

				filePath := filepath.Join(path, file.Name())
				if !file.IsDir() && filepath.Ext(file.Name()) == ".strm" {
					// Convert filePath to lowercase for case-insensitive comparison
					lowerCaseFilePath := strings.ToLower(filePath)
					if !ciKeepFiles[lowerCaseFilePath] {
						logMessage(fmt.Sprintf("Removing .strm file: %s", filePath))
						if err := os.Remove(filePath); err != nil {
							logError("Error removing file:", err)
						} else {
							removedStrmFiles++
							stats["removedStrmFiles"]++
							deletions++
						}
					} else {
						logDebug(fmt.Sprintf("Keeping .strm file: %s", filePath))
					}
				}
			}

			// Check if the directory is empty and remove it if it is
			isEmpty, err := isDirEmpty(path)
			if err != nil {
				logError("Error checking directory:", err)
				return err
			}
			if isEmpty && path != rootDir {
				logMessage(fmt.Sprintf("Removing empty directory: %s", path))
				if err := os.Remove(path); err != nil {
					logError("Error removing directory:", err)
				} else {
					removedEmptyDirs++
					stats["removedEmptyDirs"]++
				}
				return filepath.SkipDir // Skip further processing of this directory
			}
		}
		return nil
	})
}

func isDirEmpty(dir string) (bool, error) {
	logDebug(fmt.Sprintf("Checking if directory is empty: %s", dir))
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func loadConfig(configFile string) (*Config, error) {
	config := &Config{}
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		// Check if the file exists in the current working directory
		if !filepath.IsAbs(configFile) {
			currentDir, _ := os.Getwd()
			configFile = filepath.Join(currentDir, configFile)
			file, err = ioutil.ReadFile(configFile)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	err = json.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}

	// Trim spaces for excludeGroup and includeGroup
	config.ExcludeGroup = strings.TrimSpace(config.ExcludeGroup)
	config.IncludeGroup = strings.TrimSpace(config.IncludeGroup)
	config.ExcludeGroup = strings.ReplaceAll(config.ExcludeGroup, ", ", ",")
	config.IncludeGroup = strings.ReplaceAll(config.IncludeGroup, ", ", ",")

	// Ensure proper trimming and splitting of excludeGroup and includeGroup
	excludeGroups = filterEmptyStrings(strings.Split(strings.TrimSpace(config.ExcludeGroup), ","))
	includeGroups = filterEmptyStrings(strings.Split(strings.TrimSpace(config.IncludeGroup), ","))

	// Check for any invalid keys in the JSON file
	var raw map[string]interface{}
	if err := json.Unmarshal(file, &raw); err != nil {
		return nil, err
	}
	for key := range raw {
		if _, ok := configMap[key]; !ok {
			fmt.Printf("Warning: Unrecognized configuration key: %s\n", key)
		}
	}
	return config, nil
}

var configMap = map[string]struct{}{
	"name":           {},
	"logLevel":       {},
	"tvShowsDir":     {},
	"moviesDir":      {},
	"jsonURLs":       {},
	"m3uURLs":        {},
	"logFile":        {},
	"fileType":       {},
	"workingDir":     {},
	"logDir":         {},
	"downloadDir":    {},
	"retainDownload": {},
	"useGroup":       {},
	"defaultGroup":   {},
	"limitDelete":    {},
	"excludeGroup":   {},
	"includeGroup":   {},
}

func saveConfigToFile(config *Config) error {
	configPath := filepath.Join(workingDir, fmt.Sprintf("%s.json", config.Name))
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configPath, configData, 0644)
}

func processJSON(jsonURL string, index int) ([]Stream, error) {
	// Download the JSON file
	resp, err := http.Get(jsonURL)
	if err != nil {
		return nil, fmt.Errorf("error downloading json file: %v", err)
	}
	defer resp.Body.Close()

	// Read the JSON file
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading json file: %v", err)
	}

	// Save the JSON file locally
	filename := filepath.Join(downloadDir, fmt.Sprintf("GetSTRM_%d_%s.json", index, time.Now().Format("20060102_150405")))
	err = ioutil.WriteFile(filename, body, 0644)
	if err != nil {
		return nil, fmt.Errorf("error saving json file: %v", err)
	}
	logMessage(fmt.Sprintf("Saved JSON file: %s", filename))

	// Parse the JSON data
	var streams []Stream
	err = json.Unmarshal(body, &streams)
	if err != nil {
		return nil, fmt.Errorf("error parsing json file: %v", err)
	}

	return streams, nil
}

func processM3U(m3uURL string, index int, stats map[string]int) ([]Stream, error) {
	// Fetch the M3U file
	client := &http.Client{}
	req, err := http.NewRequest("GET", m3uURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for m3u file: %v", err)
	}

	// Set user agent to emulate a modern Chrome browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error downloading m3u file: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading m3u file: %v", err)
	}

	// Save the M3U file locally
	filename := filepath.Join(downloadDir, fmt.Sprintf("GetSTRM_%d_%s.m3u", index, time.Now().Format("20060102_150405")))
	err = ioutil.WriteFile(filename, body, 0644)
	if err != nil {
		return nil, fmt.Errorf("error saving m3u file: %v", err)
	}
	logMessage(fmt.Sprintf("Saved M3U file: %s", filename))

	var streams []Stream
	var currentStream *Stream

	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#EXTINF:") {
			// Parse the metadata line
			tvgName := parseTvgName(line)
			groupTitle := parseGroupTitle(line)
			currentStream = &Stream{
				TvgName:    tvgName,
				GroupTitle: groupTitle,
			}
		} else if currentStream != nil {
			// This line contains the URL
			currentStream.URL = line
			if isValidStrmType(currentStream.URL) {
				streams = append(streams, *currentStream)
			} else {
				logDebug(fmt.Sprintf("Rejected: %s", *currentStream))
				stats["rejectedFileExts"]++
			}
			currentStream = nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading M3U file: %v", err)
	}

	return streams, nil
}

func parseTvgName(line string) string {
	re := regexp.MustCompile(`tvg-name="([^"]*)"`)
	match := re.FindStringSubmatch(line)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func parseGroupTitle(line string) string {
	re := regexp.MustCompile(`group-title="([^"]*)"`)
	match := re.FindStringSubmatch(line)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func isValidStrmType(url string) bool {
	for _, ext := range fileTypes {
		if strings.HasSuffix(strings.ToLower(url), strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

func processTVShow(stream Stream, tvShowsDir string, groupTitle string, tvShowRegex *regexp.Regexp, keepFiles map[string]bool, stats map[string]int) {
	// Extract show name and season/episode info
	parts := tvShowRegex.FindString(stream.TvgName)
	if parts == "" {
		logError("Invalid TV show name format:", stream.TvgName)
		return
	}
	// Split the name and season/episode
	showName := sanitizeFileName(normalizeName(strings.SplitN(stream.TvgName, parts, 2)[0]))
	seasonEpisode := parts
	season := seasonEpisode[:3] // Extract "Sxx"

	// Create directory structure
	var showDir string
	if useGroup == 1 {
		showDir = filepath.Join(tvShowsDir, groupTitle, showName, season)
	} else {
		showDir = filepath.Join(tvShowsDir, showName, season)
	}
	if _, err := os.Stat(showDir); os.IsNotExist(err) {
		logMessage(fmt.Sprintf("Creating directory: %s", showDir))
		err = os.MkdirAll(showDir, os.ModePerm)
		if err != nil {
			logError(fmt.Sprintf("Error creating directory: %s", showDir), err)
			return
		}
		createdDirs++
		stats["createdDirs"]++
	}

	// Create or update .strm file
	strmFilePath := filepath.Join(showDir, sanitizeFileName(normalizeName(stream.TvgName))+".strm")
	keepFiles[strmFilePath] = true
	createOrUpdateStrmFile(strmFilePath, stream.URL)
	keptStrmFiles++
	stats["keptStrmFiles"]++
	logDebug(fmt.Sprintf("Keep STRM File: %s", strmFilePath))
}

func processMovie(stream Stream, moviesDir string, groupTitle string, keepFiles map[string]bool, stats map[string]int) {
	// Create directory structure
	var movieDir string
	if useGroup == 1 {
		movieDir = filepath.Join(moviesDir, groupTitle, sanitizeFileName(normalizeName(stream.TvgName)))
	} else {
		movieDir = filepath.Join(moviesDir, sanitizeFileName(normalizeName(stream.TvgName)))
	}
	if _, err := os.Stat(movieDir); os.IsNotExist(err) {
		logMessage(fmt.Sprintf("Creating directory: %s", movieDir))
		err = os.MkdirAll(movieDir, os.ModePerm)
		if err != nil {
			logError(fmt.Sprintf("Error creating directory: %s", movieDir), err)
			return
		}
		createdDirs++
		stats["createdDirs"]++
	}

	// Create or update .strm file
	strmFilePath := filepath.Join(movieDir, sanitizeFileName(normalizeName(stream.TvgName))+".strm")
	keepFiles[strmFilePath] = true
	createOrUpdateStrmFile(strmFilePath, stream.URL)
	keptStrmFiles++
	stats["keptStrmFiles"]++
	logDebug(fmt.Sprintf("Keep STRM File: %s", strmFilePath))
}

func createOrUpdateStrmFile(filePath, url string) {
	file, err := os.Create(filePath)
	if err != nil {
		logError("Error creating .strm file:", err)
		return
	}
	defer file.Close()

	_, err = file.WriteString(url)
	if err != nil {
		logError("Error writing to .strm file:", err)
	}
}

func sanitizeFileName(name string) string {
	// Replace invalid characters with an underscore and trim spaces
	invalidChars := regexp.MustCompile(`[<>:"/\\|?*[\]#%&{}$!'"+=@~` + "`" + `]`)
	sanitized := strings.TrimSpace(invalidChars.ReplaceAllString(name, "_"))
	// Remove trailing dots and colons, then replace multiple dots with single dot
	sanitized = strings.TrimRight(sanitized, "_.:")
	sanitized = strings.ReplaceAll(sanitized, "..", ".")
	sanitized = strings.ReplaceAll(sanitized, ".", "_") // Replace remaining dots with underscores
	sanitized = strings.TrimRight(sanitized, " ")
	return sanitized
}

func normalizeName(name string) string {
	// Remove ellipses and normalize spaces
	name = strings.ReplaceAll(name, "...", "")
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, ".", " ")
	return strings.TrimSpace(name)
}

func printStatistics(stats map[string]int) {
	statMessage := fmt.Sprintf("Statistics:\nDirectories created: %d\n.strm files kept: %d\n.strm files removed: %d\nEmpty directories removed: %d\nProcessed JSON URLs: %d\nProcessed M3U URLs: %d\nRejected file extensions: %d\n",
		stats["createdDirs"], stats["keptStrmFiles"], stats["removedStrmFiles"], stats["removedEmptyDirs"], stats["processedJsonURLs"], stats["processedM3UURLs"], stats["rejectedFileExts"])
	logMessage(statMessage)
}

func logError(messages ...interface{}) {
	if logLevel > 0 {
		logMessage(fmt.Sprintln(messages...))
	}
}

func logDebug(message string) {
	if logLevel == 3 {
		logMessage(fmt.Sprint("DEBUG: ", message))
	}
}

func logMessage(message string) {
	timestampedMessage := fmt.Sprintf("%s %s", time.Now().Format(time.RFC3339), message)
	fmt.Println(timestampedMessage) // Print to console
	if logFileHandle != nil {
		logFileHandle.WriteString(timestampedMessage + "\n") // Write to log file
	}
}

func showHelp() {
	fmt.Println(`

Usage: GetSTRM [options]
Options:
  -config string
        Path to the configuration file
  -name string
        Name to be printed in the log file and used for config file creation
  -logLevel int
        Log level: 0 = silent, 1 = basic information, 3 = all debug and errors (default: 1)
  -tvShowsDir string
        Directory for TV shows (required)
  -moviesDir string
        Directory for movies (required)
  -jsonURL string
        URL of the JSON file (can be specified multiple times, required if no m3uURLs)
  -m3u string
        URL of the M3U file (can be specified multiple times, required if no jsonURLs)
  -logFile string
        Name of the log file (default: vod_log.txt)
  -fileType string
        Comma separated list of valid strm file types (default: avi,flv,m4v,mkv,mkv2,mkv5,mkvv,mp4,mp41,mp42,mp44,mpg,wmv)
  -workingDir string
        Working directory (default: current working directory)
  -logDir string
        Directory for log files (default: workingDir/Log)
  -retainDownload int
        Set to 1 to keep downloaded files, 0 to delete (default: 0)
  -downloadDir string
        Directory to keep downloaded files (overrides default)
  -limitDelete int
        Maximum number of .strm files to delete (default: 25)
  -useGroup int
        Set to 1 to use group title in directory structure, 0 to not use (default: 0)
  -defaultGroup string
        Default group title if useGroup is set and group is not specified (default: Dummy)
  -excludeGroup string
        Comma separated list of groups to exclude
  -includeGroup string
        Comma separated list of groups to include
  -version
        Display the version information
  -help
        Show help message

Examples:
  go run script_name.go -config "config.json"
  go run script_name.go -name MyStreamApp -tvShowsDir /path/to/tvshows -moviesDir /path/to/movies -jsonURL http://example.com/file.json -logFile mylog.txt`)
}

func filterEmptyStrings(slice []string) []string {
	var result []string
	for _, str := range slice {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}

func hasCommonElement(slice1, slice2 []string) bool {
	set := make(map[string]struct{})
	for _, val := range slice1 {
		set[val] = struct{}{}
	}
	for _, val := range slice2 {
		if _, found := set[val]; found {
			return true
		}
	}
	return false
}

func createDefaultConfig() {
	workingDir, _ := os.Getwd()
	tvShowsDir := filepath.Join(workingDir, "vod_tv")
	moviesDir := filepath.Join(workingDir, "vod_movie")
	downloadDir := filepath.Join(workingDir, "Download")
	logDir := filepath.Join(workingDir, "Log")

	// Create directories if they do not exist
	if _, err := os.Stat(tvShowsDir); os.IsNotExist(err) {
		os.MkdirAll(tvShowsDir, os.ModePerm)
		fmt.Println("Created directory:", tvShowsDir)
	}
	if _, err := os.Stat(moviesDir); os.IsNotExist(err) {
		os.MkdirAll(moviesDir, os.ModePerm)
		fmt.Println("Created directory:", moviesDir)
	}

	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		os.MkdirAll(downloadDir, os.ModePerm)
		fmt.Println("Created directory:", downloadDir)
	}
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		os.MkdirAll(logDir, os.ModePerm)
		fmt.Println("Created directory:", logDir)
	}

	defaultConfig := &Config{
		Name:           "Default",
		LogLevel:       1,
		RetainDownload: 0,
		LimitDelete:    25,
		DownloadDir:    downloadDir,
		TvShowsDir:     tvShowsDir,
		MoviesDir:      moviesDir,
		JsonURLs:       []string{},
		M3UURLs:        []string{},
		LogFile:        "vod_log.txt",
		FileType:       "avi,flv,m4v,mkv,mkv2,mkv5,mkvv,mp4,mp41,mp42,mp44,mpg,wmv",
		WorkingDir:     workingDir,
		LogDir:         logDir,
		UseGroup:       0,
		DefaultGroup:   "Dummy",
		ExcludeGroup:   "",
		IncludeGroup:   "",
	}

	defaultConfigPath := "sample_config.json"
	if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
		configData, _ := json.MarshalIndent(defaultConfig, "", "  ")
		ioutil.WriteFile(defaultConfigPath, configData, 0644)
		fmt.Println("Default configuration created at sample_config.json")
	}
}

func init() {
	// Only create a default config if no command line args or config file is provided
	if len(os.Args) == 1 {
		createDefaultConfig()
	}
}

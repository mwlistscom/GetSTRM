# GetSTRM

GetSTRM manages the directory structure for your Emby/JellyFin strm files. By default it adds the dicrectory structure for every STRM in your m3u IPTV playlist.

The source can be 1 or more m3u or STRM export from RockMyM3u.com or multiple of both.

GetSTRM will delete empty directories and streams that are no longer in the provider list.

For this reason, do not use this with existing directories with media files.

Need help ? Contact us on https://www.facebook.com/rockmym3u or email help@rockmym3u.com

# Setup

See release for binaries for Windows or Linux

Alternately install golang and execute  "go run GetSTRM.go"

Without any parameters GetSTRM will create a sample sample\_config.json file, rename this file and edit

the variables within. This is a JSON standard file do not add comments.

# Usage

Usage: GetSTRM [options]

Options:

- config string

Path to the configuration file

- name string

Name to be printed in the log file and used for config file creation

- logLevel int

Log level: 0 = silent, 1 = basic information, 3 = all debug and errors (default: 1)

- tvShowsDir string

Directory for TV shows (required)

- moviesDir string

Directory for movies (required)

- jsonURL string

URL of the JSON file (can be specified multiple times, required if no m3uURLs)

- m3u string

URL of the M3U file (can be specified multiple times, required if no jsonURLs)

- logFile string

Name of the log file (default: vod\_log.txt)

- fileType string

Comma separated list of valid strm file types (default: avi,flv,m4v,mkv,mkv2,mkv5,mkvv,mp4,mp41,mp42,mp44,mpg,wmv)

- workingDir string

Working directory (default: current working directory)

- logDir string

Directory for log files (default: workingDir/Log)

- retainDownload int

Set to 1 to keep downloaded files, 0 to delete (default: 0)

- downloadDir string

Directory to keep downloaded files (overrides default)

- excludeGroup string

Comma separated list of groups to exclude (default: null)

- includeGroup string

Comma separated list of groups to include (default: null)

- limitDelete int

Maximum number of .strm files to delete (default: 25)

- version

Display the version information

- help

Show help message

# License

Copyright (c) 2024 Jules Potvin

This software is licensed under the Creative Commons Attribution-NonCommercial 4.0 International License.

You may use, distribute, and modify this code under the terms of the license.

To view a copy of this license, visit http://creativecommons.org/licenses/by-nc/4.0/

This code is provided "as is", without warranty of any kind, express or implied, including but not limited to the

warranties of merchantability, fitness for a particular purpose and noninfringement. In no event shall the

authors or copyright holders be liable for any claim, damages or other liability, whether in an action of contract,

tort or otherwise, arising from, out of or in connection with the software or the use or other dealings in the software.

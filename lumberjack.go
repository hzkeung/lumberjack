// Package lumberjack provides a rolling logger.
//
// Note that this is v3.0 of lumberjack.
//
// Lumberjack is intended to be one part of a logging infrastructure.
// It is not an all-in-one solution, but instead is a pluggable
// component at the bottom of the logging stack that simply controls the files
// to which logs are written.
//
// Lumberjack plays well with any logging package that can write to an
// io.Writer, including the standard library's log package.
//
// Lumberjack assumes that only one process is writing to the output files.
// Using the same lumberjack configuration from multiple processes on the same
// machine will result in improper behavior. Letting outside processes write to
// or manipulate the file that lumberjack writes to will also result in improper
// behavior.
package lumberjack

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	backupTimeFormat = "20060102150405"
	compressSuffix   = ".gz"
	defaultMaxSize   = 100 * 1024 * 1024
)

type constError string

func (c constError) Error() string {
	return string(c)
}

// ErrWriteTooLong indicates that a single write that is longer than the max
// size allowed in a single file.
const ErrWriteTooLong = constError("write exceeds max file length")

func getIntervalTime(rotateType RotateType, rotateTime int) []int {
	var intervalTime []int = []int{}
	if rotateTime == 0 {
		return intervalTime
	}
	switch rotateType {
	case RotateMinute:
		for i := rotateTime; i < 60; i += rotateTime {
			intervalTime = append(intervalTime, i)
		}
		return intervalTime
	case RotateHourly:
		for i := rotateTime; i < 24-rotateTime; i += rotateTime {
			intervalTime = append(intervalTime, i)
		}
		return intervalTime
	}
	return intervalTime
}

// Options represents optional behavior you can specify for a new Roller.

// NewRoller returns a new Roller.
//
// If the file exists and is less than maxSize bytes, lumberjack will open and
// append to that file. If the file exists and its size is >= maxSize bytes, the
// file is renamed by putting the current time in a timestamp in the name
// immediately before the file's extension (or the end of the filename if
// there's no extension). A new log file is then created using original
// filename.
//
// An error is returned if a file cannot be opened or created, or if maxsize is
// 0 or less.
func NewRoller(filename string, opt *Options) (*Roller, error) {
	if filename == "" {
		return nil, errors.New("filename cannot be empty")
	}
	r := &Roller{
		filename:            filename,
		maxSize:             defaultMaxSize,
		disableRotateByTime: true,
	}
	if opt != nil {
		r.maxAge = opt.MaxAge
		r.maxBackups = opt.MaxBackups
		r.localTime = opt.LocalTime
		r.compress = opt.Compress
		r.maxSize = opt.MaxSize
		if !IsLegalRotateType(opt.RotateType) {
			return nil, errors.New("rotate type is illegal")
		}
		r.disableRotateByTime = (opt.RotateType == RotateDateNotNeed || opt.RotateType == RotateSize)
		if !r.disableRotateByTime {
			if opt.RotateTime == 0 {
				opt.RotateTime = 1
			}
			if opt.RotateType == RotateHourly && 24/opt.RotateTime <= 0 {
				return nil, errors.New("RotateTime must be a divisor of 24")
			}
			if opt.RotateType == RotateDaily {
				r.maxAge = 24 * time.Hour * opt.MaxAge
			} else if opt.RotateType == RotateHourly {
				r.maxAge = time.Hour * opt.MaxAge
			} else if opt.RotateType == RotateMinute {
				r.maxAge = time.Minute * opt.MaxAge
			}
		}
		r.rotateType = opt.RotateType
		r.rotateTime = opt.RotateTime
		r.Hook = opt.Hook
	}
	if r.maxSize <= 0 {
		r.maxSize = defaultMaxSize
	}
	err := r.openExistingOrNew(0)
	if err != nil {
		return nil, fmt.Errorf("can't open file: %w", err)
	}
	return r, nil
}

// Roller wraps a file, intercepting its writes to control its size, rolling the
// old file over to a different name before writing to a new one.
//
// Whenever a write would cause the current log file exceed maxSize bytes, the
// current file is closed, renamed, and a new log file created with the original
// name. Thus, the filename you give Roller is always the "current" log file.
//
// Backups use the log file name given to Roller, in the form
// `name-timestamp.ext` where name is the filename without the extension,
// timestamp is the time at which the log was rotated formatted with the
// time.Time format of `2006-01-02T15-04-05.000` and the extension is the
// original extension. For example, if your Roller.Filename is
// `/var/log/foo/server.log`, a backup created at 6:30pm on Nov 11 2016 would
// use the filename `/var/log/foo/server-2016-11-04T18-30-00.000.log`
//
// # Cleaning Up Old Log Files
//
// Whenever a new logfile gets created, old log files may be deleted. The most
// recent files according to the encoded timestamp will be retained, up to a
// number equal to MaxBackups (or all of them if MaxBackups is 0). Any files
// with an encoded timestamp older than MaxAge days are deleted, regardless of
// MaxBackups. Note that the time encoded in the timestamp is the rotation
// time, which may differ from the last time that file was written to.
//
// If MaxBackups and MaxAge are both 0, no old log files will be deleted.
type Roller struct {
	// filename is the file to write logs to.  Backup log files will be retained
	// in the same directory.  It uses <processname>-lumberjack.log in
	// os.TempDir() if empty.
	filename string

	// maxSize is the maximum size in bytes of the log file before it gets
	// rotated.
	maxSize int64

	// maxAge is the maximum time to retain old log files based on the timestamp
	// encoded in their filename. The default is not to remove old log files
	// based on age.
	maxAge time.Duration

	// maxBackups is the maximum number of old log files to retain.  The default
	// is to retain all old log files (though MaxAge may still cause them to get
	// deleted.)
	maxBackups int

	// localTime determines if the time used for formatting the timestamps in
	// backup files is the computer's local time.  The default is to use UTC
	// time.
	localTime bool

	// compress determines if the rotated log files should be compressed
	// using gzip. The default is not to perform compression.
	compress bool

	rotateType RotateType
	// if RotateType is RotateHourly, need make (24%RotateTime==0 && 24/RotateTime > 0)
	rotateTime          uint // unit depends on RotateType
	disableRotateByTime bool

	size int64
	file *os.File
	mu   sync.Mutex

	millCh    chan bool
	startMill sync.Once

	createdTimestamp int64
	remainSeconds    int64

	Hook *Hook
}

var (
	// currentTime exists so it can be mocked out by tests.
	currentTime = time.Now

	// os_Stat exists so it can be mocked out by tests.
	osStat = os.Stat
)

// Write implements io.Writer.  If a write would cause the log file to be larger
// than MaxSize, the file is closed, renamed to include a timestamp of the
// current time, and a new log file is created using the original log file name.
// If the length of the write is greater than MaxSize, an error is returned.
func (r *Roller) Write(p []byte) (n int, err error) {
	writeLen := int64(len(p))
	if r.disableRotateByTime && writeLen > r.maxSize {
		return 0, fmt.Errorf(
			"write length %d, max size %d: %w", writeLen, r.maxSize, ErrWriteTooLong,
		)
	}

	defer r.mu.Unlock()
	r.mu.Lock()
	// 时间切割优先
	if r.disableRotateByTime {
		if r.size+writeLen > r.maxSize {
			if err := r.rotate(); err != nil {
				return 0, err
			}
		}
	} else if r.needRotateByDate() {
		if err := r.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = r.file.Write(p)
	r.size += int64(n)

	return n, err
}

// Close implements io.Closer, and closes the current logfile.
func (r *Roller) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.close()
}

// close closes the file if it is open.
func (r *Roller) close() error {
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}

// Rotate causes Logger to close the existing log file and immediately create a
// new one.  This is a helper function for applications that want to initiate
// rotations outside of the normal rotation rules, such as in response to
// SIGHUP.  After rotating, this initiates compression and removal of old log
// files according to the configuration.
func (r *Roller) Rotate() error {
	defer r.mu.Unlock()
	r.mu.Lock()
	return r.rotate()
}

// rotate closes the current file, moves it aside with a timestamp in the name,
// (if it exists), opens a new file with the original filename, and then runs
// post-rotation processing and removar.
func (r *Roller) rotate() error {
	if err := r.close(); err != nil {
		return err
	}
	if err := r.openNew(); err != nil {
		return err
	}
	r.mill()
	return nil
}

// openNew opens a new log file for writing, moving any old log file out of the
// way.  This methods assumes the file has already been closed.
func (r *Roller) openNew() error {
	err := os.MkdirAll(r.dir(), 0755)
	if err != nil {
		return fmt.Errorf("can't make directories for new logfile: %w", err)
	}

	name := r.newFilename()
	mode := os.FileMode(0644)
	info, err := osStat(name)
	if err == nil {
		// Copy the mode off the old logfile.
		mode = info.Mode()
		// move the existing file
		dateStr := currentTime().Format(backupTimeFormat)
		if !r.localTime {
			dateStr = currentTime().UTC().Format(backupTimeFormat)
		}
		if r.rotateType == "hourly" {
			dateStr = currentTime().Add(-1 * time.Hour).Format("20060102-15")
		}
		if r.rotateType == "hourly" && !r.localTime {
			dateStr = currentTime().Add(-1 * time.Hour).UTC().Format("20060102-15")
		}
		if r.rotateType == "daily" {
			dateStr = currentTime().Add(-1 * time.Hour).Format("20060102")
		}
		if r.rotateType == "daily" && !r.localTime {
			dateStr = currentTime().Add(-1 * time.Hour).UTC().Format("20060102")
		}
		newname := backupName(name, dateStr)
		if err := os.Rename(name, newname); err != nil {
			return fmt.Errorf("can't rename log file: %w", err)
		}
		if r.Hook != nil {
			go r.Hook.AfterRotate(newname)
		}
		// this is a no-op anywhere but linux
		if err := chown(name, info); err != nil {
			return err
		}
	}

	// we use truncate here because this should only get called when we've moved
	// the file ourselves. if someone else creates the file in the meantime,
	// just wipe out the contents.
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("can't open new logfile: %w", err)
	}
	r.file = f
	r.size = 0
	r.calRotateCycle()
	return nil
}

// backupName creates a new filename from the given name, inserting a timestamp
// between the filename and the extension, using the local time if requested
// (otherwise UTC).
func backupName(name string, timestamp string) string {
	dir := filepath.Dir(name)
	filename := filepath.Base(name)
	ext := filepath.Ext(filename)
	prefix := filename[:len(filename)-len(ext)]
	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", prefix, timestamp, ext))
}

// openExistingOrNew opens the logfile if it exists and if the current write
// would not put it over MaxSize.  If there is no such file or the write would
// put it over the MaxSize, a new file is created.
func (r *Roller) openExistingOrNew(writeLen int64) error {
	r.mill()

	filename := r.newFilename()
	info, err := osStat(filename)
	if os.IsNotExist(err) {
		return r.openNew()
	}
	if err != nil {
		return fmt.Errorf("error getting log file info: %w", err)
	}
	if r.disableRotateByTime {
		if info.Size()+writeLen >= r.maxSize {
			return r.rotate()
		}
	}
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		// if we fail to open the old log file for some reason, just ignore
		// it and open a new log file.
		return r.openNew()
	}
	r.file = file
	r.size = info.Size()
	r.calRotateCycle()
	return nil
}

// newFilename generates the name of the logfile from the current time.
func (r *Roller) newFilename() string {
	if r.filename != "" {
		return r.filename
	}
	name := filepath.Base(os.Args[0]) + "-lumberjack.log"
	return filepath.Join(os.TempDir(), name)
}

// millRunOnce performs compression and removal of stale log files.
// Log files are compressed if enabled via configuration and old log
// files are removed, keeping at most r.MaxBackups files, as long as
// none of them are older than MaxAge.
func (r *Roller) millRunOnce() error {
	if r.maxBackups == 0 && r.maxAge == 0 && !r.compress {
		return nil
	}

	files, err := r.oldLogFiles()
	if err != nil {
		return err
	}

	var compress, remove []logInfo

	if r.maxBackups > 0 && r.maxBackups < len(files) {
		preserved := make(map[string]bool)
		var remaining []logInfo
		for _, f := range files {
			// Only count the uncompressed log file or the
			// compressed log file, not both.
			fn := f.Name()
			if strings.HasSuffix(fn, compressSuffix) {
				fn = fn[:len(fn)-len(compressSuffix)]
			}
			preserved[fn] = true

			if len(preserved) > r.maxBackups {
				remove = append(remove, f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining
	}
	if r.maxAge > 0 {
		cutoff := currentTime().Add(-1 * r.maxAge)

		var remaining []logInfo
		for _, f := range files {
			if f.timestamp.Before(cutoff) {
				remove = append(remove, f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining
	}

	if r.compress {
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), compressSuffix) {
				compress = append(compress, f)
			}
		}
	}

	for _, f := range remove {
		errRemove := os.Remove(filepath.Join(r.dir(), f.Name()))
		if err == nil && errRemove != nil {
			err = errRemove
		}
	}
	for _, f := range compress {
		fn := filepath.Join(r.dir(), f.Name())
		errCompress := compressLogFile(fn, fn+compressSuffix)
		if err == nil && errCompress != nil {
			err = errCompress
		}
	}

	return err
}

// millRun runs in a goroutine to manage post-rotation compression and removal
// of old log files.
func (r *Roller) millRun() {
	for range r.millCh {
		// what am I going to do, log this?
		_ = r.millRunOnce()
	}
}

// mill performs post-rotation compression and removal of stale log files,
// starting the mill goroutine if necessary.
func (r *Roller) mill() {
	r.startMill.Do(func() {
		r.millCh = make(chan bool, 1)
		go r.millRun()
	})
	select {
	case r.millCh <- true:
	default:
	}
}

// oldLogFiles returns the list of backup log files stored in the same
// directory as the current log file, sorted by ModTime
func (r *Roller) oldLogFiles() ([]logInfo, error) {
	files, err := ioutil.ReadDir(r.dir())
	if err != nil {
		return nil, fmt.Errorf("can't read log file directory: %s", err)
	}
	logFiles := []logInfo{}

	prefix, ext := r.prefixAndExt()

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if t, err := r.timeFromName(f.Name(), prefix, ext); err == nil {
			logFiles = append(logFiles, logInfo{t, f})
			continue
		}
		if t, err := r.timeFromName(f.Name(), prefix, ext+compressSuffix); err == nil {
			logFiles = append(logFiles, logInfo{t, f})
			continue
		}
		// error parsing means that the suffix at the end was not generated
		// by lumberjack, and therefore it's not a backup file.
	}

	sort.Sort(byFormatTime(logFiles))

	return logFiles, nil
}

// timeFromName extracts the formatted time from the filename by stripping off
// the filename's prefix and extension. This prevents someone's filename from
// confusing time.parse.
func (r *Roller) timeFromName(filename, prefix, ext string) (time.Time, error) {
	if !strings.HasPrefix(filename, prefix) {
		return time.Time{}, errors.New("mismatched prefix")
	}
	if !strings.HasSuffix(filename, ext) {
		return time.Time{}, errors.New("mismatched extension")
	}
	ts := filename[len(prefix) : len(filename)-len(ext)]
	return time.Parse(backupTimeFormat, ts)
}

// dir returns the directory for the current filename.
func (r *Roller) dir() string {
	return filepath.Dir(r.newFilename())
}

// prefixAndExt returns the filename part and extension part from the Logger's
// filename.
func (r *Roller) prefixAndExt() (prefix, ext string) {
	filename := filepath.Base(r.newFilename())
	ext = filepath.Ext(filename)
	prefix = filename[:len(filename)-len(ext)] + "-"
	return prefix, ext
}

func (r *Roller) calRotateCycle() {
	if r.disableRotateByTime {
		return
	}
	n := currentTime()
	r.createdTimestamp = n.Unix()
	r.remainSeconds = calRemainderSecondToNextRotateTime(n, r.rotateType, int(r.rotateTime), r.localTime)
}

func (r *Roller) needRotateByDate() bool {
	if r.disableRotateByTime {
		return false
	}
	return currentTime().Unix()-r.createdTimestamp >= r.remainSeconds
}

// 根据当前时间，周期单位 ，周期数量 计算举例下个周期剩余的秒数
func calRemainderSecondToNextRotateTime(now time.Time, rotateType RotateType, rotateTime int, isLocal bool) int64 {
	local := time.Local
	if !isLocal {
		local = time.UTC
	}
	// 以天为周期
	if rotateType == RotateDaily {
		rotateDate := now.Add(time.Hour * time.Duration(int64(rotateTime)*24))
		y, m, d := rotateDate.Date()
		if !isLocal {
			y, m, d = rotateDate.UTC().Date()
		}
		// 重置为0点
		zeroClock := time.Date(y, m, d, 0, 0, 0, 0, local)
		return zeroClock.Unix() - now.Unix()
	}
	// 以小时为周期
	if rotateType == RotateHourly {
		currentHour := now.Hour()
		if !isLocal {
			currentHour = now.UTC().Hour()
		}
		// 下一个切割时间点
		nextRotateHour := ((currentHour / rotateTime) + 1) * rotateTime
		// 如果下一个时间切割点大于24小时，那么第二天的0点就是下一个切割点
		rotateDate := now
		if nextRotateHour >= 24 {
			rotateDate = rotateDate.Add(time.Hour * 24)
			nextRotateHour = 0
		}
		y, m, d := rotateDate.Date()
		// UTC
		if !isLocal {
			y, m, d = rotateDate.UTC().Date()
		}
		zeroClock := time.Date(y, m, d, nextRotateHour, 0, 0, 0, local)
		return zeroClock.Unix() - now.Unix()
	}

	// 以分钟为周期
	if rotateType == RotateMinute {
		rotateInterval := getIntervalTime(RotateMinute, rotateTime)
		currentMinute := now.Minute()
		if !isLocal {
			currentMinute = now.UTC().Minute()
		}
		var nextMinute int
		for _, point := range rotateInterval {
			if currentMinute < point {
				nextMinute = point
				break
			}
		}
		// 下一个切割时间点
		rotateDate := now
		// 如果没有配到任何切割点，那么下一个切割点就是下一个小时的0分
		if nextMinute == 0 {
			rotateDate = rotateDate.Add(time.Hour)
			nextMinute = 0
		}
		y, m, d := rotateDate.Date()
		h := rotateDate.Hour()
		// // UTC
		if !isLocal {
			y, m, d = rotateDate.UTC().Date()
			h = rotateDate.UTC().Hour()
		}
		zeroClock := time.Date(y, m, d, h, nextMinute, 0, 0, local)
		return zeroClock.Unix() - now.Unix()
	}
	return 0

}

// compressLogFile compresses the given log file, removing the
// uncompressed log file if successfur.
func compressLogFile(src, dst string) (err error) {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer f.Close()

	fi, err := osStat(src)
	if err != nil {
		return fmt.Errorf("failed to stat log file: %v", err)
	}

	if err := chown(dst, fi); err != nil {
		return fmt.Errorf("failed to chown compressed log file: %v", err)
	}

	// If this file already exists, we presume it was created by
	// a previous attempt to compress the log file.
	gzf, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode())
	if err != nil {
		return fmt.Errorf("failed to open compressed log file: %v", err)
	}
	defer gzf.Close()

	gz := gzip.NewWriter(gzf)

	defer func() {
		if err != nil {
			os.Remove(dst)
			err = fmt.Errorf("failed to compress log file: %v", err)
		}
	}()

	if _, err := io.Copy(gz, f); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := gzf.Close(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		return err
	}

	return nil
}

// logInfo is a convenience struct to return the filename and its embedded
// timestamp.
type logInfo struct {
	timestamp time.Time
	os.FileInfo
}

// byFormatTime sorts by newest time formatted in the name.
type byFormatTime []logInfo

func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byFormatTime) Len() int {
	return len(b)
}

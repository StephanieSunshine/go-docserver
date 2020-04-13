package main

import (
    "fmt"
    "net/http"
    "regexp"
    "os"
    "log"
    "github.com/VictoriaMetrics/fastcache"
    "io/ioutil"
    "bytes"
)

/* go-docserver can be added to a .server folder in your static root */
const DOCROOT         = "../"

var re_FixIndex       = regexp.MustCompile(`\/$`)
var re_FixPathEscape  = regexp.MustCompile(`\.{2,}`)
var re_FixHidden      = regexp.MustCompile(`(^\.|\/\.)`)

var re_IsCSS          = regexp.MustCompile(`(?i)\.css$`)
var re_IsJS           = regexp.MustCompile(`(?i)\.js$`)
var re_IsHTML         = regexp.MustCompile(`(?i)\.html$`)
var re_IsWOFF         = regexp.MustCompile(`(?i)\.woff$`)

var mtime_lookup      = fastcache.New(64)
var data_lookup       = fastcache.New(1024)

func main() {
    http.HandleFunc("/", DocServer)
    http.ListenAndServe(":8080", nil)
}

func DocServer(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path[1:]

    /* if someone tries to access a hidden file or tries to directory escape, just send them a 404 */
    if (re_FixPathEscape.Match([]byte(path)) || re_FixHidden.Match([]byte(path))) {
      log.Println("Bad path: ", path)
      http.NotFound(w, r)
      return
    }

    /* if no file in path add index.html */
    if (re_FixIndex.Match([]byte(path)) || path == "") {
      path += "index.html"
    }

    stat, err := os.Stat(DOCROOT+path); if (err != nil || stat.IsDir()) {
      log.Println("File not found: ", DOCROOT+path)
      http.NotFound(w, r)
      return
    }

    encoded_modtime, err := stat.ModTime().GobEncode(); if err != nil {
      log.Println("Time Error")
      http.Error(w, "Time Error", http.StatusInternalServerError)
      return
    }
    log.Println(r.Header.Get("If-None-Match")," == ", fmt.Sprintf(`"%x"`, encoded_modtime))
    if r.Header.Get("If-None-Match") == fmt.Sprintf(`"%x"`, encoded_modtime) {
      log.Println("Request: ", DOCROOT+path, " Client Match. Skipping")
      w.WriteHeader(http.StatusNotModified)
      return
    }

    log.Println("Serving: ", DOCROOT+path)

    saved_modtime, has := mtime_lookup.HasGet(nil, []byte(DOCROOT+path))

    if re_IsCSS.Match([]byte(path)) {
      w.Header().Set("Content-Type", "text/css")
    } else if re_IsJS.Match([]byte(path)) {
      w.Header().Set("Content-Type", "text/javascript")
    } else if re_IsHTML.Match([]byte(path)) {
      w.Header().Set("Content-Type", "text/html")
    } else if re_IsWOFF.Match([]byte(path)) {
      w.Header().Set("Content-Type", "font/woff")
    }

    w.Header().Set("Etag", fmt.Sprintf(`"%x"`, encoded_modtime))
    w.Header().Set("Cache-Control", "max-age=2592000")

    if (has && bytes.Compare(encoded_modtime, saved_modtime) == 0){
      log.Println("From Cache")
      fmt.Fprint(w, string(data_lookup.GetBig(nil, []byte(DOCROOT+path))))
    }else{
      file, err := ioutil.ReadFile(DOCROOT+path); if err != nil {
        log.Println("File error: ", DOCROOT+path, err)
        http.NotFound(w, r)
        return
      }
      log.Println("From Disk")
      mtime_lookup.Set([]byte(DOCROOT+path), encoded_modtime)
      data_lookup.SetBig([]byte(DOCROOT+path), file)
      fmt.Fprint(w, string(file))
    }
}

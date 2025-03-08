package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

var pointCache = make(map[uuid.UUID]int)

func main() {
    var use_logging bool
    flag.BoolVar(&use_logging, "logging", false, "enable logging for all requests")
    var port int
    flag.IntVar(&port, "port", 8080, "port used for the web server (uint16)")
    if port < 1 || port > 65535 {
        log.Fatal("specified port number must be between 0 and 65535 (uint16)")
    }
    flag.Parse()

    r := mux.NewRouter()
    r.HandleFunc("/receipts/process", process).
      Methods("POST")
    r.HandleFunc("/receipts/{id}/points", getPoints).
      Methods("GET")

    if use_logging {
        log.Println("using logging")
        http.Handle("/", logRequestHandler(r))
    } else {
        http.Handle("/", r)
    }

    log.Println("Starting on port: ", port)
    log.Fatal(http.ListenAndServe(fmt.Sprint(":", port), nil))
}

func process(w http.ResponseWriter, r *http.Request) {
    invalid := func() {
        w.WriteHeader(400)
        w.Write([]byte("The receipt is invalid. Please verify input."))
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        invalid()
        return
    }

    receipt := Receipt{}

    err = json.Unmarshal(body, &receipt)
    if err != nil {
        invalid()
        return
    }

    valid := receipt.verify()
    if !valid {
        invalid()
        return
    }

    receiptID := uuid.NewSHA1(uuid.NameSpaceOID, body)

    response := struct {
        Id string `json:"id"`
    }{
        receiptID.String(),
    }

    responseJSON, err := json.Marshal(response)
    if err != nil {
        invalid()
        return
    }

    if _, exists := pointCache[receiptID]; !exists {
        p := receipt.calculatePoints()
        pointCache[receiptID] = p
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(responseJSON)
}

func getPoints(w http.ResponseWriter, r *http.Request) {
    notFound := func() {
        w.WriteHeader(404)
        w.Write([]byte("No receipt found for that id"))
    }

    vars := mux.Vars(r)
    id := vars["id"]

    receiptID, err := uuid.Parse(id)
    if err != nil {
        notFound()
        return
    }

    p, ok := pointCache[receiptID]
    if !ok {
        notFound()
        return
    }

    response := struct {
        Points int `json:"points"`
    }{
        p,
    }

    responseJSON, err := json.Marshal(response)
    if err != nil {
        notFound()
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(responseJSON)
}

func logRequestHandler(h http.Handler) http.Handler {
    fn := func(w http.ResponseWriter, r *http.Request) {
        h.ServeHTTP(w, r)
        uri := r.URL.String()
        method := r.Method
        headers := r.Header
        headerBuilder := strings.Builder{}
        headerBuilder.WriteString("headers: ")
        for key, value := range headers {
            headerBuilder.WriteString(fmt.Sprintf("'%s : %v', ", key, value))
        }
        logHTTPReq(uri, method, headerBuilder.String())
    }

    return http.HandlerFunc(fn)
}

func logHTTPReq(s ...string) {
    log.Println("New request received:")
    for _, v := range s {
        if strings.TrimSpace(v) == "" { continue }
        log.Println(v)
    }
}
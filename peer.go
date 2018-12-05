package main

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io/ioutil"
    "log"
    "math/rand"
    "net/http"
    "os"
    "os/signal"
    "strconv"
    "sync"
    "time"
)

type configuration struct {
    Peers []string `json:"peers"`
}

type Identification struct {
   IP    string
}

func readConfigFile(path string) configuration {
    file, err := os.Open(path)
    if err != nil {
        log.Fatalf("Cannot find configuration file %s", path)
    }
    defer file.Close()

    conf := new(configuration)
    decoder := json.NewDecoder(file)
    err = decoder.Decode(&conf)
    if err != nil {
        log.Fatalf("Cannot read configuration file %s", path)
    }

    return *conf
}


func ping(res http.ResponseWriter, req *http.Request) {
    fmt.Fprint(res, "Active")
}

func sendRequest(peerURL string) (string, error) {
    client := &http.Client{
        Timeout: time.Duration(2 * time.Second),
    }
    request, _ := http.NewRequest("GET", peerURL, nil)
    request.Header.Set("Accept", "text/plain; charset=utf-8")
    response, err := client.Do(request)

    if err != nil {
        return "", err
    } else {
        body, err := ioutil.ReadAll(response.Body)
        if err != nil {
            log.Println(err.Error())
        }
        return string(body), nil
    }
    return "", errors.New("Something went terribly wrong?")
}

func pingAll(res http.ResponseWriter, req *http.Request) {
    var peersToRemove []string
    for _, peer := range config.Peers {
        peerURL := fmt.Sprintf("%s/state", peer)
        fmt.Println(peerURL)
        msg, err := sendRequest(peerURL)
        if err != nil {
            log.Println(err)
            peersToRemove = append(peersToRemove, peer)
        } else {
            log.Println(msg)
        }
    }

    conf := configuration{
        Peers: peersToRemove,
    }
    confAsBytes, _ := json.Marshal(conf)
    fmt.Fprintln(res, string(confAsBytes))
}

func getNumber(res http.ResponseWriter, req *http.Request) {
    if req.Method == "POST" {
        // Test for example with:
        // curl -X POST http://192.168.20.2:8080/msg -d '5'

        body, err := ioutil.ReadAll(req.Body)
        if err != nil {
            log.Println(err.Error())
        }
        number, err := strconv.Atoi(string(body))
        if err != nil {
            log.Println("Ups... cannot get the number")
        }
        internalSum += number
    } else if req.Method == "GET" {
        fmt.Fprint(res, internalSum)
    }
}

func keepBusy() {
    randomPeerIdx := rand.Intn(len(config.Peers))
    randomNumber := strconv.Itoa(rand.Intn(100))
    fmt.Println("Sending", randomNumber, "to", config.Peers[randomPeerIdx])
    _, err := http.Post(config.Peers[randomPeerIdx]+"/msg", "text/plain", bytes.NewBuffer([]byte(randomNumber)))
    if err != nil {
        log.Println(err)
    }
}

func addPeers(res http.ResponseWriter, req *http.Request) {
    if req.Method == "POST" {
        body, err := ioutil.ReadAll(req.Body)
        if err != nil {
            log.Println(err.Error())
        }
        var peer_single = string(body)
        
        print(peer_single)
    }
}

func removePeers(res http.ResponseWriter, req *http.Request) {
    // TODO: One of the exercises
}

var pathToConfig = "peers.json"
var config = readConfigFile(pathToConfig)
var internalSum = 0

func main() {
    rand.Seed(time.Now().UnixNano())

    // equivalent to Python's if not os.path.exists(filename)
    if _, err := os.Stat(pathToConfig); os.IsNotExist(err) {
        log.Fatalf("Config file %s does not exist", pathToConfig)
    }
    http.HandleFunc("/state", ping)
    http.HandleFunc("/excludes", pingAll)
    http.HandleFunc("/msg", getNumber)
    http.HandleFunc("/newPeers", addPeers)
    http.HandleFunc("/garbagePeers", removePeers)

    // The following calls the keepBusy func every so and so many seconds
    // see: https://stackoverflow.com/questions/43002163/run-function-only-once-for-specific-time-golang
    ticker := time.NewTicker(time.Duration(rand.Intn(10)) * time.Second)
    go func(ticker *time.Ticker) {
        for {
            select {
            case <-ticker.C:
                keepBusy()
            }
        }
    }(ticker)

    // Create a new server and set timeout values.
    server := http.Server{
        Addr: ":8080",
    }

    // We want to report the listener is closed.
    var wg sync.WaitGroup
    wg.Add(1)

    // Start the listener.
    go func() {
        log.Println("listener : Listening on localhost:8080")
        log.Println("listener :", server.ListenAndServe())
        wg.Done()
    }()

    // Listen for an interrupt signal from the OS. Use a buffered
    // channel because of how the signal package is implemented.
    osSignals := make(chan os.Signal, 1)
    signal.Notify(osSignals, os.Interrupt)

    // Wait for a signal to shutdown.
    <-osSignals

    // Create a context to attempt a graceful 5 second shutdown.
    const timeout = 5 * time.Second
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // Attempt the graceful shutdown by closing the listener and
    // completing all inflight requests.
    if err := server.Shutdown(ctx); err != nil {
        log.Printf("shutdown : Graceful shutdown did not complete in %v : %v", timeout, err)

        // Looks like we timedout on the graceful shutdown. Kill it hard.
        if err := server.Close(); err != nil {
            log.Printf("shutdown : Error killing server : %v", err)
        }
    }

    // Wait for the listener to report it is closed.
    wg.Wait()
    log.Println("main : Completed")
}
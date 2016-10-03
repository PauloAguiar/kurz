package kurz

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"codec"

	"github.com/gorilla/mux"
	godis "github.com/simonz05/godis/redis"
)

const (
	// special key in redis, that is our global counter
	counterConst = "__counter__"
	httpConst    = "http"
)

type Kurz struct {
	redis        *godis.Client
	fileNotFound string
	hostname     string
	proto        string
}

type KurzUrl struct {
	Key          string
	ShortUrl     string
	LongUrl      string
	CreationDate int64
	Clicks       int64
}

// Converts the KurzUrl to JSON.
func (k KurzUrl) Json() []byte {
	b, _ := json.Marshal(k)
	return b
}

func NewKurz(redis *godis.Client, hostname string, fileNotFound string) *Kurz {
	var kurz *Kurz
	kurz = new(Kurz)

	kurz.redis = redis
	kurz.fileNotFound = fileNotFound
	kurz.proto = httpConst
	kurz.hostname = hostname
	return kurz
}

// Creates a new KurzUrl instance. The Given key, shorturl and longurl will
// be used. Clicks will be set to 0 and CreationDate to time.Nanoseconds()
func newKurzUrl(key, shorturl, longurl string) *KurzUrl {
	kurl := new(KurzUrl)
	kurl.CreationDate = time.Now().UnixNano()
	kurl.Key = key
	kurl.LongUrl = longurl
	kurl.ShortUrl = shorturl
	kurl.Clicks = 0
	return kurl
}

// stores a new KurzUrl for the given key, shorturl and longurl. Existing
// ones with the same url will be overwritten
func (k *Kurz) store(key, shorturl, longurl string) *KurzUrl {
	kurl := newKurzUrl(key, shorturl, longurl)
	go k.redis.Hset(kurl.Key, "LongUrl", kurl.LongUrl)
	go k.redis.Hset(kurl.Key, "ShortUrl", kurl.ShortUrl)
	go k.redis.Hset(kurl.Key, "CreationDate", kurl.CreationDate)
	go k.redis.Hset(kurl.Key, "Clicks", kurl.Clicks)
	return kurl
}

// loads a KurzUrl instance for the given key. If the key is
// not found, os.Error is returned.
func (k *Kurz) load(key string) (*KurzUrl, error) {
	if ok, _ := k.redis.Hexists(key, "ShortUrl"); ok {
		kurl := new(KurzUrl)
		kurl.Key = key
		reply, _ := k.redis.Hmget(key, "LongUrl", "ShortUrl", "CreationDate", "Clicks")
		kurl.LongUrl, kurl.ShortUrl, kurl.CreationDate, kurl.Clicks =
			reply.Elems[0].Elem.String(), reply.Elems[1].Elem.String(),
			reply.Elems[2].Elem.Int64(), reply.Elems[3].Elem.Int64()
		return kurl, nil
	}
	return nil, errors.New("unknown key: " + key)
}

// function to display the info about a KurzUrl given by it's Key
func (k *Kurz) Info(w http.ResponseWriter, r *http.Request) {
	short := mux.Vars(r)["short"]
	if strings.HasSuffix(short, "+") {
		short = strings.Replace(short, "+", "", 1)
	}

	kurl, err := k.load(short)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(kurl.Json())
		io.WriteString(w, "\n")
	} else {
		http.Redirect(w, r, k.fileNotFound, http.StatusNotFound)
	}
}

// function to resolve a shorturl and redirect
func (k *Kurz) Resolve(w http.ResponseWriter, r *http.Request) {

	short := mux.Vars(r)["short"]
	kurl, err := k.load(short)
	if err == nil {
		go k.redis.Hincrby(kurl.Key, "Clicks", 1)
		http.Redirect(w, r, kurl.LongUrl, http.StatusMovedPermanently)
	} else {
		http.Redirect(w, r, k.fileNotFound, http.StatusMovedPermanently)
	}
}

// function to shorten and store a url
func (k *Kurz) Shorten(w http.ResponseWriter, r *http.Request) {
	host := k.hostname

	leUrl := r.FormValue("url")
	theUrl, err := isValidUrl(string(leUrl))
	if err == nil {
		ctr, _ := k.redis.Incr(counterConst)
		encoded := codec.Encode(ctr)
		location := fmt.Sprintf("%s://%s/%s", k.proto, host, encoded)
		k.store(encoded, location, theUrl.String())

		home := r.FormValue("home")
		if home != "" {
			http.Redirect(w, r, "/", http.StatusMovedPermanently)
		} else {
			// redirect to the info page
			http.Redirect(w, r, location+"+", http.StatusMovedPermanently)
		}
	} else {
		http.Redirect(w, r, k.fileNotFound, http.StatusNotFound)
	}
}

//Returns a json array with information about the last shortened urls. If data
// is a valid integer, that's the amount of data it will return, otherwise
// a maximum of 10 entries will be returned.
func (k *Kurz) Latest(w http.ResponseWriter, r *http.Request) {
	data := mux.Vars(r)["data"]
	howmany, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		howmany = 10
	}
	c, _ := k.redis.Get(counterConst)

	last := c.Int64()
	upTo := (last - howmany)

	w.Header().Set("Content-Type", "application/json")

	var kurls = []*KurzUrl{}

	for i := last; i > upTo && i > 0; i -= 1 {
		kurl, err := k.load(codec.Encode(i))
		if err == nil {
			kurls = append(kurls, kurl)
		}
	}
	s, _ := json.Marshal(kurls)
	w.Write(s)
}

// Determines if the string rawurl is a valid URL to be stored.
func isValidUrl(rawurl string) (u *url.URL, err error) {
	if len(rawurl) == 0 {
		return nil, errors.New("empty url")
	}
	// XXX this needs some love...
	if !strings.HasPrefix(rawurl, httpConst) {
		rawurl = fmt.Sprintf("%s://%s", httpConst, rawurl)
	}
	return url.Parse(rawurl)
}

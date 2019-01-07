package lgtR

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/monkeydioude/tools/cache"
	"github.com/turnage/graw/reddit"
)

type hotData map[string]uint64
type hotCache cache.FileCache

var agentFilePath string

type Hot struct {
	BaseCachePath string
	Bot           reddit.Bot
	WatchTimer    time.Duration
}

type Watcher struct {
	Cache      *cache.FileCache
	SubPath    string
	NewPostCb  func(*reddit.Post)
	WatchTimer time.Duration
	Bot        reddit.Bot
	Cancel     context.CancelFunc
	Start      bool
}

func cachePathFromSub(sub string) string {
	return strings.Replace(sub, "/", "", -1)
}

func (h *Hot) WatchMe(sub string, cb func(*reddit.Post)) *Watcher {
	// if room does not exist blabla
	hd := make(hotData)
	hc := cache.NewFileCache(h.BaseCachePath+cachePathFromSub(sub), &hd)
	hc.Parse()

	ctx, cancelFunc := context.WithCancel(context.Background())

	w := &Watcher{
		Cache:      hc,
		SubPath:    "/r/" + sub + "/hot",
		NewPostCb:  cb,
		Bot:        h.Bot,
		WatchTimer: h.WatchTimer,
		Cancel:     cancelFunc,
		Start:      false,
	}

	go w.watch(ctx)
	return w
}

func (w *Watcher) compareAndPostData(trial []*reddit.Post) bool {
	from := *(w.Cache.GetData().(*hotData))
	hasModif := false
	for _, post := range trial {
		if _, ok := from[post.ID]; !ok {
			if w.Start == true {
				w.NewPostCb(post)
			}
			hasModif = true
			from[post.ID] = post.CreatedUTC
		}
	}
	w.Start = true
	return hasModif
}

func (w *Watcher) watch(ctx context.Context) {
	fmt.Printf("Checking %s !\n", w.SubPath)

	harvest, err := w.Bot.ListingWithParams(w.SubPath, map[string]string{"limit": "5"})
	if err != nil {
		fmt.Println("Failed to fetch: ", err)
		return
	}

	if w.compareAndPostData(harvest.Posts) {
		if err := w.Cache.Write(w.Cache.Data, 0); err != nil {
			fmt.Println("Failed to store in cache: ", err)
			return
		}
	}

	select {
	case <-time.After(w.WatchTimer):
		w.watch(ctx)
	case <-ctx.Done():
		return
	}

}

func init() {
	agentFilePath = os.Getenv("AGENT_FILE")
	if agentFilePath == "" {
		log.Fatal("[ERR ] valid AGENT_FILE env var must be given")
	}
}

func New(baseCachePath string, watchTimer time.Duration) *Hot {
	bot, err := reddit.NewBotFromAgentFile(agentFilePath, 0)
	if err != nil {
		fmt.Println("Failed to fetch: ", err)
		return nil
	}
	os.MkdirAll(baseCachePath, 0766)
	return &Hot{
		Bot:           bot,
		BaseCachePath: baseCachePath,
		WatchTimer:    watchTimer,
	}
}

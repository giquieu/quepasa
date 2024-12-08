package models

import (
	"encoding/json"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nocodeleaks/quepasa/whatsapp"
	log "github.com/sirupsen/logrus"
)

type QpCache struct {
	counter  atomic.Uint64
	cacheMap sync.Map
}

func (source *QpCache) Count() uint64 {
	return source.counter.Load()
}

func (source *QpCache) SetAny(key string, value interface{}, expiration time.Duration) {
	item := QpCacheItem{key, value, time.Now().Add(expiration)}
	source.SetCacheItem(item, "any")
}

func (source *QpCache) SetCacheItem(item QpCacheItem, from string) {
	previous, loaded := source.cacheMap.Swap(item.Key, item)
	if loaded {
		prevItem := previous.(QpCacheItem)
		log.Warnf("[%s][%s] updating cache item ...", item.Key, from)
		log.Warnf("[%s][%s] old type: %s, %v", item.Key, from, reflect.TypeOf(prevItem.Value), prevItem.Value)
		log.Warnf("[%s][%s] new type: %s, %v", item.Key, from, reflect.TypeOf(item.Value), item.Value)

		if oldJson, ok := prevItem.Value.(*whatsapp.WhatsappMessage); ok {
			b, err := json.Marshal(oldJson)
			if err == nil {
				log.Warnf("[%s][%s] old as json: %s", item.Key, from, b)
			}
		}
		if newJson, ok := item.Value.(*whatsapp.WhatsappMessage); ok {
			b, err := json.Marshal(newJson)
			if err == nil {
				log.Warnf("[%s][%s] new as json: %s", item.Key, from, b)
			}
		}

		log.Warnf("[%s][%s] equals: %v", item.Key, from, item.Value == prevItem.Value)

	} else {
		source.counter.Add(1)
	}
}

func (source *QpCache) GetAny(key string) (interface{}, bool) {
	if val, ok := source.cacheMap.Load(key); ok {
		item := val.(QpCacheItem)
		if time.Now().Before(item.Expiration) {
			return item.Value, true
		} else {
			source.DeleteByKey(key)
		}
	}
	return nil, false
}

func (source *QpCache) Delete(item QpCacheItem) {
	source.DeleteByKey(item.Key)
}

func (source *QpCache) DeleteByKey(key string) {
	_, loaded := source.cacheMap.LoadAndDelete(key)
	if loaded {
		source.counter.Add(^uint64(0))
	}
}

// gets a copy as array of cached items
func (source *QpCache) GetSliceOfCachedItems() (items []QpCacheItem) {
	source.cacheMap.Range(func(key, value any) bool {
		item := value.(QpCacheItem)
		items = append(items, item)
		return true
	})
	return items
}

// get a copy as array of cached items, ordered by expiration
func (source *QpCache) GetOrdered() (items []QpCacheItem) {

	// filling array
	items = source.GetSliceOfCachedItems()

	// ordering
	sort.Sort(QpCacheOrdering(items))
	return
}

// remove old ones, by timestamp, until a maximum length
func (source *QpCache) CleanUp(max uint64) {
	if max > 0 {
		length := source.counter.Load()
		amount := length - max
		if amount > 0 {
			items := source.GetOrdered()
			for i := 0; i < int(amount); i++ {
				source.DeleteByKey(items[i].Key)
			}
		}
	}
}

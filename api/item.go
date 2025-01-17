package api

import (
	"strings"
	"sync"
	"unicode"

	"github.com/RasmusLindroth/go-mastodon"
	"github.com/RasmusLindroth/tut/util"
)

var id uint = 0
var idMux sync.Mutex

func newID() uint {
	idMux.Lock()
	defer idMux.Unlock()
	id = id + 1
	return id
}

type Item interface {
	ID() uint
	Type() MastodonType
	ToggleSpoiler()
	ShowSpoiler() bool
	Raw() interface{}
	URLs() ([]util.URL, []mastodon.Mention, []mastodon.Tag, int)
	Filtered() (bool, string)
	Pinned() bool
}

type filtered struct {
	inUse bool
	name  string
}

func NewStatusItem(item *mastodon.Status, filters []*mastodon.Filter, timeline string, pinned bool) (sitem Item) {
	filtered := filtered{inUse: false}
	if item == nil {
		return &StatusItem{id: newID(), item: item, showSpoiler: false, filtered: filtered, pinned: pinned}
	}
	s := util.StatusOrReblog(item)
	content := s.Content
	if s.Sensitive {
		content += "\n" + s.SpoilerText
	}
	content = strings.ToLower(content)
	for _, f := range filters {
		apply := false
		for _, c := range f.Context {
			if timeline == c {
				apply = true
				break
			}
		}
		if !apply {
			continue
		}
		if f.WholeWord {
			lines := strings.Split(content, "\n")
			var stripped []string
			for _, l := range lines {
				var words []string
				words = append(words, strings.Split(l, " ")...)
				for _, w := range words {
					ns := strings.TrimSpace(w)
					ns = strings.TrimFunc(ns, func(r rune) bool {
						return !unicode.IsLetter(r) && !unicode.IsNumber(r)
					})
					stripped = append(stripped, ns)
				}
			}
			filter := strings.Split(strings.ToLower(f.Phrase), " ")
			for i := 0; i+len(filter)-1 < len(stripped); i++ {
				if strings.ToLower(f.Phrase) == strings.Join(stripped[i:i+len(filter)], " ") {
					filtered.inUse = true
					filtered.name = f.Phrase
					break
				}
			}
		} else {
			if strings.Contains(s.Content, strings.ToLower(f.Phrase)) {
				filtered.inUse = true
				filtered.name = f.Phrase
			}
			if strings.Contains(s.SpoilerText, strings.ToLower(f.Phrase)) {
				filtered.inUse = true
				filtered.name = f.Phrase
			}
		}
		if filtered.inUse {
			break
		}
	}
	sitem = &StatusItem{id: newID(), item: item, showSpoiler: false, filtered: filtered, pinned: pinned}
	return sitem
}

type StatusItem struct {
	id          uint
	item        *mastodon.Status
	showSpoiler bool
	filtered    filtered
	pinned      bool
}

func (s *StatusItem) ID() uint {
	return s.id
}

func (s *StatusItem) Type() MastodonType {
	return StatusType
}

func (s *StatusItem) ToggleSpoiler() {
	s.showSpoiler = !s.showSpoiler
}

func (s *StatusItem) ShowSpoiler() bool {
	return s.showSpoiler
}

func (s *StatusItem) Raw() interface{} {
	return s.item
}

func (s *StatusItem) URLs() ([]util.URL, []mastodon.Mention, []mastodon.Tag, int) {
	status := s.item
	if status.Reblog != nil {
		status = status.Reblog
	}
	_, urls := util.CleanHTML(status.Content)
	if status.Sensitive {
		_, u := util.CleanHTML(status.SpoilerText)
		urls = append(urls, u...)
	}

	realUrls := []util.URL{}
	for _, url := range urls {
		isNotMention := true
		for _, mention := range status.Mentions {
			if mention.URL == url.URL {
				isNotMention = false
			}
		}
		if isNotMention {
			realUrls = append(realUrls, url)
		}
	}

	length := len(realUrls) + len(status.Mentions) + len(status.Tags)
	return realUrls, status.Mentions, status.Tags, length
}

func (s *StatusItem) Filtered() (bool, string) {
	return s.filtered.inUse, s.filtered.name
}

func (s *StatusItem) Pinned() bool {
	return s.pinned
}

func NewUserItem(item *User, profile bool) Item {
	return &UserItem{id: newID(), item: item, profile: profile}
}

type UserItem struct {
	id      uint
	item    *User
	profile bool
}

func (u *UserItem) ID() uint {
	return u.id
}

func (u *UserItem) Type() MastodonType {
	if u.profile {
		return ProfileType
	}
	return UserType
}

func (u *UserItem) ToggleSpoiler() {
}

func (u *UserItem) ShowSpoiler() bool {
	return false
}

func (u *UserItem) Raw() interface{} {
	return u.item
}

func (u *UserItem) URLs() ([]util.URL, []mastodon.Mention, []mastodon.Tag, int) {
	user := u.item.Data
	var urls []util.URL
	user.Note, urls = util.CleanHTML(user.Note)
	for _, f := range user.Fields {
		_, fu := util.CleanHTML(f.Value)
		urls = append(urls, fu...)
	}

	return urls, []mastodon.Mention{}, []mastodon.Tag{}, len(urls)
}

func (s *UserItem) Filtered() (bool, string) {
	return false, ""
}

func (u *UserItem) Pinned() bool {
	return false
}

func NewNotificationItem(item *mastodon.Notification, user *User, filters []*mastodon.Filter) (nitem Item) {
	status := NewStatusItem(item.Status, filters, "notifications", false)
	nitem = &NotificationItem{
		id:          newID(),
		item:        item,
		showSpoiler: false,
		user:        NewUserItem(user, false),
		status:      status,
	}

	return nitem
}

type NotificationItem struct {
	id          uint
	item        *mastodon.Notification
	showSpoiler bool
	status      Item
	user        Item
}

type NotificationData struct {
	Item   *mastodon.Notification
	Status Item
	User   Item
}

func (n *NotificationItem) ID() uint {
	return n.id
}

func (n *NotificationItem) Type() MastodonType {
	return NotificationType
}

func (n *NotificationItem) ToggleSpoiler() {
	n.showSpoiler = !n.showSpoiler
}

func (n *NotificationItem) ShowSpoiler() bool {
	return n.showSpoiler
}

func (n *NotificationItem) Raw() interface{} {
	return &NotificationData{
		Item:   n.item,
		Status: n.status,
		User:   n.user,
	}
}

func (n *NotificationItem) URLs() ([]util.URL, []mastodon.Mention, []mastodon.Tag, int) {
	return nil, nil, nil, 0
}

func (n *NotificationItem) Filtered() (bool, string) {
	return false, ""
}

func (n *NotificationItem) Pinned() bool {
	return false
}

func NewListsItem(item *mastodon.List) Item {
	return &ListItem{id: newID(), item: item, showSpoiler: true}
}

type ListItem struct {
	id          uint
	item        *mastodon.List
	showSpoiler bool
}

func (s *ListItem) ID() uint {
	return s.id
}

func (s *ListItem) Type() MastodonType {
	return ListsType
}

func (s *ListItem) ToggleSpoiler() {
}

func (s *ListItem) ShowSpoiler() bool {
	return true
}

func (s *ListItem) Raw() interface{} {
	return s.item
}

func (s *ListItem) URLs() ([]util.URL, []mastodon.Mention, []mastodon.Tag, int) {
	return nil, nil, nil, 0
}

func (s *ListItem) Filtered() (bool, string) {
	return false, ""
}

func (n *ListItem) Pinned() bool {
	return false
}

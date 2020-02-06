package spotify

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"golang.org/x/oauth2/clientcredentials"

	"github.com/zmb3/spotify"
)

type Spotify interface {
	AddToPlaylist(title, artist string) error
	Skip()
	DontSkip()
}

type SpotifyClient struct {
	spotify   spotify.Client
	Playlist  *spotify.FullPlaylist
	skipVoted bool
	skipVotes int
	keepVotes int
	skipTimer *time.Timer
}

type SpotifyClientOpts struct {
	ClientID     string
	Secret       string
	PlaylistName string // one or the other between these 2
	PlaylistID   string
}

func NewSpotifyClient(opts SpotifyClientOpts) (Spotify, error) {
	config := &clientcredentials.Config{
		ClientID:     opts.ClientID,
		ClientSecret: opts.Secret,
		TokenURL:     spotify.TokenURL,
	}
	token, err := config.Token(context.Background())
	if err != nil {
		return nil, err
	}
	client := spotify.Authenticator{}.NewClient(token)

	spotClient := &SpotifyClient{
		spotify: client,
	}
	user, err := client.CurrentUser()
	if err != nil {
		return nil, err
	}
	if opts.PlaylistID == "" {
		// we assume you want to create a new one - could offer the option to give playlist ID instead
		playlist, err := client.CreatePlaylistForUser(user.ID, opts.PlaylistName, "echo office playlist", false)
		if err != nil {
			return nil, err
		}
		spotClient.Playlist = playlist
	} else {
		playlist, err := client.GetPlaylist(spotify.ID(opts.PlaylistID))
		if err != nil {
			return nil, err
		}
		spotClient.Playlist = playlist
	}

	return spotClient, nil
}

func (s *SpotifyClient) AddToPlaylist(title, artist string) error {
	searchQuery := title + " - " + artist

	results, err := s.spotify.Search(url.QueryEscape(searchQuery), spotify.SearchTypeTrack)
	if err != nil {
		return err
	}
	if results.Tracks != nil {
		if len(results.Tracks.Tracks) == 1 {
			// queue it
			// :( https://github.com/spotify/web-api/issues/462
			trackID := results.Tracks.Tracks[0].ID
			if s.isTrackInPlaylist(trackID) {
				return fmt.Errorf("Track already in playlist")
			}
			_, err := s.spotify.AddTracksToPlaylist(spotify.ID(s.Playlist.ID))
			if err != nil {
				return err
			}
		}
		// give options
	}
	return nil
}

func (s *SpotifyClient) isTrackInPlaylist(trackID spotify.ID) bool {
	inPage := false
	for true {
		inPage = isTrackInPage(trackID, s.Playlist.Tracks)
		if inPage {
			return true
		}
		err := s.spotify.NextPage(s.Playlist.Tracks)
		if err != nil && err == spotify.ErrNoMorePages {
			return false
		} else if err != nil {
			// TODO log stuff here
			return false
		}
	}
	return false
}

func isTrackInPage(trackID spotify.ID, page spotify.PlaylistTrackPage) bool {
	for _, t := range page.Tracks {
		if t.Track.ID == trackID {
			return true
		}
	}
	return false
}

func (s *SpotifyClient) Skip() {
	if !s.skipVoted {
		s.skipTimer = time.NewTimer(time.Second * 10)
		s.skipVoted = true
	}
	s.skipVotes += 1
}

func (s *SpotifyClient) DontSkip() {
	s.keepVotes += 1
}

func (s *SpotifyClient) TimerExpired() error {
	if s.skipVotes > s.keepVotes {
		return s.spotify.Next()
	}
	return fmt.Errorf("Not enough skip votes!")
}

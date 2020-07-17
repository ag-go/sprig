package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
	"git.sr.ht/~whereswaldon/forest-go/grove"
	"git.sr.ht/~whereswaldon/forest-go/store"
	"git.sr.ht/~whereswaldon/sprig/ds"
	sprigTheme "git.sr.ht/~whereswaldon/sprig/widget/theme"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

func main() {
	go func() {
		w := app.NewWindow(app.Title("Sprig"))
		if err := eventLoop(w); err != nil {
			log.Fatalf("exiting due to error: %v", err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func eventLoop(w *app.Window) error {
	address := flag.String("address", "", "arbor relay address to connect to")
	profile := flag.Bool("profile", false, "log profiling data")
	flag.Parse()
	dataDir, err := app.DataDir()
	if err != nil {
		log.Printf("failed finding application data dir: %v", err)
	}

	appState, err := NewAppState(dataDir)
	if err != nil {
		return err
	}
	if *address != "" {
		appState.Settings.Address = *address
		appState.RestartWorker()
	}

	viewManager := NewViewManager(w, appState.Theme, *profile)
	viewManager.RegisterView(ConnectFormID, NewConnectFormView(&appState.Settings, &appState.ArborState, appState.Theme))
	viewManager.RegisterView(CommunityMenuID, NewCommunityMenuView(&appState.Settings, &appState.ArborState, appState.Theme))
	viewManager.RegisterView(ReplyViewID, NewReplyListView(&appState.Settings, &appState.ArborState, appState.Theme))
	viewManager.RegisterView(IdentityFormID, NewIdentityFormView(&appState.Settings, &appState.ArborState, appState.Theme))
	viewManager.RegisterView(ConsentViewID, NewConsentView(&appState.Settings, &appState.ArborState, appState.Theme))
	if appState.Settings.AcknowledgedNoticeVersion < NoticeVersion {
		viewManager.RequestViewSwitch(ConsentViewID)
	} else if appState.Settings.Address == "" {
		viewManager.RequestViewSwitch(ConnectFormID)
	} else {
		viewManager.RequestViewSwitch(CommunityMenuID)
	}

	notifMgr, err := NewNotificationManager(appState)

	appState.SubscribableStore.SubscribeToNewMessages(func(n forest.Node) {
		w.Invalidate()
		notifMgr.HandleNode(n)
	})
	var ops op.Ops
	for {
		switch event := (<-w.Events()).(type) {
		case system.DestroyEvent:
			return event.Err
		case system.ClipboardEvent:
			viewManager.HandleClipboard(event.Text)
		case *system.CommandEvent:
			if event.Type == system.CommandBack {
				viewManager.HandleBackNavigation(event)
			}
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, event)
			layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return sprigTheme.DrawRect(gtx, appState.Theme.Background.Dark, f32.Pt(float32(gtx.Constraints.Max.X), float32(gtx.Constraints.Max.Y)), 0)
				}),
				layout.Stacked(func(gtx C) D {
					return layout.Inset{
						Bottom: event.Insets.Bottom,
						Left:   event.Insets.Left,
						Right:  event.Insets.Right,
						Top:    event.Insets.Top,
					}.Layout(gtx, func(gtx C) D {
						return layout.Stack{}.Layout(gtx,
							layout.Expanded(func(gtx C) D {
								return sprigTheme.DrawRect(gtx, appState.Theme.Background.Default, f32.Pt(float32(gtx.Constraints.Max.X), float32(gtx.Constraints.Max.Y)), 0)
							}),
							layout.Stacked(viewManager.Layout),
						)
					})
				}),
			)
			event.Frame(gtx.Ops)
		}
	}
}

type AppState struct {
	Settings
	ArborState
	*sprigTheme.Theme
	DataDir      string
	TimeLaunched uint64
}

func NewAppState(dataDir string) (*AppState, error) {
	dataDir = filepath.Join(dataDir, "sprig")
	// NOTE: time has to be converted from nanoseconds to milliseconds for Arbor's specifications
	TimeLocation, _ := time.LoadLocation("UTC")
	TimeLaunched := uint64(time.Now().In(TimeLocation).UnixNano() / 1000000)
	var baseStore forest.Store
	var err error
	if err = os.MkdirAll(dataDir, 0770); err != nil {
		log.Printf("couldn't create app data dir: %v", err)
	}
	if dataDir != "" {
		grovePath := filepath.Join(dataDir, "grove")
		if err := os.MkdirAll(grovePath, 0770); err != nil {
			log.Printf("unable to create directory for grove: %v", err)
		}
		baseStore, err = grove.New(grovePath)
		if err != nil {
			log.Printf("unable to create grove (falling back to in-memory): %v", err)
		}
	}
	if baseStore == nil {
		baseStore = store.NewMemoryStore()
	}
	archive := store.NewArchive(baseStore)
	rl, err := ds.NewReplyList(archive)
	if err != nil {
		return nil, fmt.Errorf("failed to construct replylist: %w", err)
	}
	cl, err := ds.NewCommunityList(archive)
	if err != nil {
		return nil, fmt.Errorf("failed to construct communitylist: %w", err)
	}
	appState := &AppState{
		ArborState: ArborState{
			SubscribableStore: archive,
			ReplyList:         rl,
			CommunityList:     cl,
		},
		DataDir:      dataDir,
		Theme:        sprigTheme.New(),
		TimeLaunched: TimeLaunched,
	}
	appState.Settings.dataDir = dataDir
	jsonSettings, err := ioutil.ReadFile(appState.Settings.SettingsFile())
	if err != nil {
		log.Printf("failed to load settings: %v", err)
	} else {
		if err = json.Unmarshal(jsonSettings, &appState.Settings); err != nil {
			log.Printf("couldn't parse json settings: %v", err)
		}
	}
	if appState.Settings.Address != "" {
		appState.RestartWorker()
	}
	if err := appState.Settings.DiscoverIdentities(); err != nil {
		log.Printf("failed to automatically discover existing identities: %v", err)
	}

	return appState, nil
}

func (a *AppState) RestartWorker() {
	a.ArborState.RestartWorker(a.Settings.Address)
}

func (a *AppState) CreateIdentity(name string) {
	if err := a.Settings.CreateIdentity(name); err != nil {
		log.Printf("failed creating identity: %v", err)
		return
	}
	identity, err := a.Settings.Identity()
	if err != nil {
		log.Printf("failed looking up identity immediately after generating it: %v", err)
		return
	}
	if err := a.ArborState.SubscribableStore.Add(identity); err != nil {
		log.Printf("failed adding identity to store: %v", err)
		return
	}
}

func (s *Settings) DiscoverIdentities() error {
	idsDir, err := os.Open(s.IdentitiesDir())
	if err != nil {
		return fmt.Errorf("failed opening identities directory: %w", err)
	}
	names, err := idsDir.Readdirnames(0)
	if err != nil {
		return fmt.Errorf("failed listing identities directory: %w", err)
	}
	name := names[0]
	id := &fields.QualifiedHash{}
	err = id.UnmarshalText([]byte(name))
	if err != nil {
		return fmt.Errorf("failed unmarshalling name of first identity %s: %w", name, err)
	}
	s.ActiveIdentity = id
	return nil
}

func (s Settings) SettingsFile() string {
	return filepath.Join(s.dataDir, "settings.json")
}

func (s Settings) KeysDir() string {
	return filepath.Join(s.dataDir, "keys")
}

func (s Settings) IdentitiesDir() string {
	return filepath.Join(s.dataDir, "identities")
}

func (s *Settings) Identity() (*forest.Identity, error) {
	if s.ActiveIdentity == nil {
		return nil, fmt.Errorf("no identity configured")
	}
	if s.activeIdCache != nil {
		return s.activeIdCache, nil
	}
	idData, err := ioutil.ReadFile(filepath.Join(s.IdentitiesDir(), s.ActiveIdentity.String()))
	if err != nil {
		return nil, fmt.Errorf("failed reading identity data: %w", err)
	}
	identity, err := forest.UnmarshalIdentity(idData)
	if err != nil {
		return nil, fmt.Errorf("failed decoding identity data: %w", err)
	}
	s.activeIdCache = identity
	return identity, nil
}

func (s *Settings) Signer() (forest.Signer, error) {
	if s.ActiveIdentity == nil {
		return nil, fmt.Errorf("no identity configured, therefore no private key")
	}
	var privkey *openpgp.Entity
	if s.activePrivKey != nil {
		privkey = s.activePrivKey
	} else {
		keyfilePath := filepath.Join(s.KeysDir(), s.ActiveIdentity.String())
		keyfile, err := os.Open(keyfilePath)
		if err != nil {
			return nil, fmt.Errorf("unable to read key file: %w", err)
		}
		defer keyfile.Close()
		privkey, err = openpgp.ReadEntity(packet.NewReader(keyfile))
		if err != nil {
			return nil, fmt.Errorf("unable to decode key data: %w", err)
		}
		s.activePrivKey = privkey
	}
	signer, err := forest.NewNativeSigner(privkey)
	if err != nil {
		return nil, fmt.Errorf("couldn't wrap privkey in forest signer: %w", err)
	}
	return signer, nil
}

func (s *Settings) Builder() (*forest.Builder, error) {
	id, err := s.Identity()
	if err != nil {
		return nil, err
	}
	signer, err := s.Signer()
	if err != nil {
		return nil, err
	}
	builder := forest.As(id, signer)
	return builder, nil
}

type Settings struct {
	Address        string
	ActiveIdentity *fields.QualifiedHash

	AcknowledgedNoticeVersion int

	dataDir string

	// state used for authoring messages
	activePrivKey *openpgp.Entity
	activeIdCache *forest.Identity
}

func (s *Settings) CreateIdentity(name string) (err error) {
	keysDir := s.KeysDir()
	if err := os.MkdirAll(keysDir, 0770); err != nil {
		return fmt.Errorf("failed creating key storage directory: %w", err)
	}
	keypair, err := openpgp.NewEntity(name, "sprig-generated arbor identity", "", &packet.Config{})
	if err != nil {
		return fmt.Errorf("failed generating new keypair: %w", err)
	}
	signer, err := forest.NewNativeSigner(keypair)
	if err != nil {
		return fmt.Errorf("failed wrapping keypair into Signer: %w", err)
	}
	identity, err := forest.NewIdentity(signer, name, []byte{})
	if err != nil {
		return fmt.Errorf("failed generating arbor identity from signer: %w", err)
	}
	id := identity.ID()

	keyFilePath := filepath.Join(keysDir, id.String())
	keyFile, err := os.OpenFile(keyFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0660)
	if err != nil {
		return fmt.Errorf("failed creating key file: %w", err)
	}
	defer func() {
		if err != nil {
			if err = keyFile.Close(); err != nil {
				err = fmt.Errorf("failed closing key file: %w", err)
			}
		}
	}()
	if err := keypair.SerializePrivateWithoutSigning(keyFile, nil); err != nil {
		return fmt.Errorf("failed saving private key: %w", err)
	}

	idsDir := s.IdentitiesDir()
	if err := os.MkdirAll(idsDir, 0770); err != nil {
		return fmt.Errorf("failed creating identity storage directory: %w", err)
	}
	idFilePath := filepath.Join(idsDir, id.String())

	idFile, err := os.OpenFile(idFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0660)
	if err != nil {
		return fmt.Errorf("failed creating identity file: %w", err)
	}
	defer func() {
		if err != nil {
			if err = idFile.Close(); err != nil {
				err = fmt.Errorf("failed closing identity file: %w", err)
			}
		}
	}()
	binIdent, err := identity.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed serializing new identity: %w", err)
	}
	if _, err := idFile.Write(binIdent); err != nil {
		return fmt.Errorf("failed writing identity: %w", err)
	}

	s.ActiveIdentity = id
	s.activePrivKey = keypair
	s.Persist()
	return nil
}

func (s Settings) Persist() {
	data, err := json.MarshalIndent(&s, "", "  ")
	if err != nil {
		log.Printf("couldn't marshal settings as json: %v", err)
	}
	err = ioutil.WriteFile(s.SettingsFile(), data, 0770)
	if err != nil {
		log.Printf("couldn't save settings file: %v", err)
	}
}

type ViewID int

const (
	ConnectFormID ViewID = iota
	IdentityFormID
	CommunityMenuID
	ReplyViewID
	ConsentViewID
)

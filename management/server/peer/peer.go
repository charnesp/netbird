package peer

import (
	"fmt"
	"net"
	"time"
)

// Peer represents a machine connected to the network.
// The Peer is a WireGuard peer identified by a public key
type Peer struct {
	// ID is an internal ID of the peer
	ID string `gorm:"primaryKey"`
	// AccountID is a reference to Account that this object belongs
	AccountID string `json:"-" gorm:"index;uniqueIndex:idx_peers_account_id_ip"`
	// WireGuard public key
	Key string `gorm:"index"`
	// A setup key this peer was registered with
	SetupKey string
	// IP address of the Peer
	IP net.IP `gorm:"uniqueIndex:idx_peers_account_id_ip"`
	// Meta is a Peer system meta data
	Meta PeerSystemMeta `gorm:"embedded;embeddedPrefix:meta_"`
	// Name is peer's name (machine name)
	Name string
	// DNSLabel is the parsed peer name for domain resolution. It is used to form an FQDN by appending the account's
	// domain to the peer label. e.g. peer-dns-label.netbird.cloud
	DNSLabel string
	// Status peer's management connection status
	Status *PeerStatus `gorm:"embedded;embeddedPrefix:peer_status_"`
	// The user ID that registered the peer
	UserID string
	// SSHKey is a public SSH key of the peer
	SSHKey string
	// SSHEnabled indicates whether SSH server is enabled on the peer
	SSHEnabled bool
	// LoginExpirationEnabled indicates whether peer's login expiration is enabled and once expired the peer has to re-login.
	// Works with LastLogin
	LoginExpirationEnabled bool
	// LastLogin the time when peer performed last login operation
	LastLogin time.Time
	// Indicate ephemeral peer attribute
	Ephemeral bool
	// Geo location based on connection IP
	Location Location `gorm:"embedded;embeddedPrefix:location_"`
}

type PeerStatus struct {
	// LastSeen is the last time peer was connected to the management service
	LastSeen time.Time
	// Connected indicates whether peer is connected to the management service or not
	Connected bool
	// LoginExpired
	LoginExpired bool
	// RequiresApproval indicates whether peer requires approval or not
	RequiresApproval bool
}

// Location is a geo location information of a Peer based on public connection IP
type Location struct {
	ConnectionIP net.IP // from grpc peer or reverse proxy headers depends on setup
	CountryCode  string
	CityName     string
	GeoNameID    uint // city level geoname id
}

// PeerSystemMeta is a metadata of a Peer machine system
type PeerSystemMeta struct {
	Hostname      string
	GoOS          string
	Kernel        string
	Core          string
	Platform      string
	OS            string
	OSVersion     string
	WtVersion     string
	UIVersion     string
	KernelVersion string
}

func (p PeerSystemMeta) isEqual(other PeerSystemMeta) bool {
	return p.Hostname == other.Hostname &&
		p.GoOS == other.GoOS &&
		p.Kernel == other.Kernel &&
		p.KernelVersion == other.KernelVersion &&
		p.Core == other.Core &&
		p.Platform == other.Platform &&
		p.OS == other.OS &&
		p.OSVersion == other.OSVersion &&
		p.WtVersion == other.WtVersion &&
		p.UIVersion == other.UIVersion
}

// AddedWithSSOLogin indicates whether this peer has been added with an SSO login by a user.
func (p *Peer) AddedWithSSOLogin() bool {
	return p.UserID != ""
}

// Copy copies Peer object
func (p *Peer) Copy() *Peer {
	peerStatus := p.Status
	if peerStatus != nil {
		peerStatus = p.Status.Copy()
	}
	return &Peer{
		ID:                     p.ID,
		AccountID:              p.AccountID,
		Key:                    p.Key,
		SetupKey:               p.SetupKey,
		IP:                     p.IP,
		Meta:                   p.Meta,
		Name:                   p.Name,
		DNSLabel:               p.DNSLabel,
		Status:                 peerStatus,
		UserID:                 p.UserID,
		SSHKey:                 p.SSHKey,
		SSHEnabled:             p.SSHEnabled,
		LoginExpirationEnabled: p.LoginExpirationEnabled,
		LastLogin:              p.LastLogin,
		Ephemeral:              p.Ephemeral,
		Location:               p.Location,
	}
}

// UpdateMetaIfNew updates peer's system metadata if new information is provided
// returns true if meta was updated, false otherwise
func (p *Peer) UpdateMetaIfNew(meta PeerSystemMeta) bool {
	// Avoid overwriting UIVersion if the update was triggered sole by the CLI client
	if meta.UIVersion == "" {
		meta.UIVersion = p.Meta.UIVersion
	}

	if p.Meta.isEqual(meta) {
		return false
	}
	p.Meta = meta
	return true
}

// MarkLoginExpired marks peer's status expired or not
func (p *Peer) MarkLoginExpired(expired bool) {
	newStatus := p.Status.Copy()
	newStatus.LoginExpired = expired
	if expired {
		newStatus.Connected = false
	}
	p.Status = newStatus
}

// LoginExpired indicates whether the peer's login has expired or not.
// If Peer.LastLogin plus the expiresIn duration has happened already; then login has expired.
// Return true if a login has expired, false otherwise, and time left to expiration (negative when expired).
// Login expiration can be disabled/enabled on a Peer level via Peer.LoginExpirationEnabled property.
// Login expiration can also be disabled/enabled globally on the Account level via Settings.PeerLoginExpirationEnabled.
// Only peers added by interactive SSO login can be expired.
func (p *Peer) LoginExpired(expiresIn time.Duration) (bool, time.Duration) {
	if !p.AddedWithSSOLogin() || !p.LoginExpirationEnabled {
		return false, 0
	}
	expiresAt := p.LastLogin.Add(expiresIn)
	now := time.Now()
	timeLeft := expiresAt.Sub(now)
	return timeLeft <= 0, timeLeft
}

// FQDN returns peers FQDN combined of the peer's DNS label and the system's DNS domain
func (p *Peer) FQDN(dnsDomain string) string {
	if dnsDomain == "" {
		return ""
	}
	return fmt.Sprintf("%s.%s", p.DNSLabel, dnsDomain)
}

// EventMeta returns activity event meta related to the peer
func (p *Peer) EventMeta(dnsDomain string) map[string]any {
	return map[string]any{"name": p.Name, "fqdn": p.FQDN(dnsDomain), "ip": p.IP}
}

// Copy PeerStatus
func (p *PeerStatus) Copy() *PeerStatus {
	return &PeerStatus{
		LastSeen:         p.LastSeen,
		Connected:        p.Connected,
		LoginExpired:     p.LoginExpired,
		RequiresApproval: p.RequiresApproval,
	}
}

// UpdateLastLogin and set login expired false
func (p *Peer) UpdateLastLogin() *Peer {
	p.LastLogin = time.Now().UTC()
	newStatus := p.Status.Copy()
	newStatus.LoginExpired = false
	p.Status = newStatus
	return p
}

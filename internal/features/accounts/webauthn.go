package accounts

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// ceremonyTimeout is how long the browser, and the server, give a person to use their passkey.
const ceremonyTimeout = 5 * time.Minute

// WebAuthn runs the passkey ceremonies with go-webauthn.
type WebAuthn struct {
	wa *webauthn.WebAuthn
}

// NewWebAuthn returns the ceremonies for the given sites. Each ceremony is bound to the site it starts on, so a
// passkey made on one host never works on another.
func NewWebAuthn(sites []Site) (*WebAuthn, error) {
	if len(sites) == 0 {
		return nil, errors.New("accounts: no sites for passkeys")
	}
	origins := make([]string, len(sites))
	for i, s := range sites {
		origins[i] = s.Origin
	}
	wa, err := webauthn.New(&webauthn.Config{
		RPID:          sites[0].ID,
		RPDisplayName: "Utanabi",
		RPOrigins:     origins,
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: ceremonyTimeout, TimeoutUVD: ceremonyTimeout},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: ceremonyTimeout, TimeoutUVD: ceremonyTimeout},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("accounts: set up passkeys failed (%v)", err)
	}
	return &WebAuthn{wa: wa}, nil
}

// BeginRegistration asks for a discoverable passkey (so signing in needs no username) that checks who's using it
// (Face ID, a fingerprint, or a PIN), and not one the account already has.
func (w *WebAuthn) BeginRegistration(site Site, a Account) (json.RawMessage, []byte, error) {
	user := waUser{a}
	exclude := make([]protocol.CredentialDescriptor, len(a.Passkeys))
	for i, k := range user.WebAuthnCredentials() {
		exclude[i] = k.Descriptor()
	}
	creation, session, err := w.wa.BeginRegistration(user,
		webauthn.WithRegistrationRelyingPartyID(site.ID),
		webauthn.WithRegistrationOrigin(site.Origin),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		}),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
		webauthn.WithExclusions(exclude),
		// Save it on this device (Face ID, Touch ID, Windows Hello) rather than offering another one by QR code
		// first, so making an account on a phone is one step.
		webauthn.WithPublicKeyCredentialHints([]protocol.PublicKeyCredentialHints{
			protocol.PublicKeyCredentialHintClientDevice,
		}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("accounts: begin passkey registration failed (%v)", err)
	}
	return marshalCeremony(creation, session)
}

// FinishRegistration checks the new passkey against the ceremony's challenge, site, and user.
func (w *WebAuthn) FinishRegistration(_ Site, a Account, state, response []byte) (Passkey, error) {
	var session webauthn.SessionData
	if err := json.Unmarshal(state, &session); err != nil {
		return Passkey{}, ErrPasskeyRejected
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return Passkey{}, ErrPasskeyRejected
	}
	cred, err := w.wa.CreateCredential(waUser{a}, session, parsed)
	if err != nil || !cred.Flags.UserVerified {
		return Passkey{}, ErrPasskeyRejected
	}
	return fromCredential(*cred), nil
}

// BeginSignIn asks for any of this site's passkeys, checked by who's using it. A conditional sign-in shows them in the
// browser's autofill instead of a prompt.
func (w *WebAuthn) BeginSignIn(site Site, conditional bool) (json.RawMessage, []byte, error) {
	mediation := protocol.MediationDefault
	if conditional {
		mediation = protocol.MediationConditional
	}
	assertion, session, err := w.wa.BeginDiscoverableMediatedLogin(mediation,
		webauthn.WithLoginRelyingPartyID(site.ID),
		webauthn.WithLoginOrigin(site.Origin),
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("accounts: begin passkey sign-in failed (%v)", err)
	}
	return marshalCeremony(assertion, session)
}

// FinishSignIn checks the passkey's signature over the ceremony's challenge. A passkey whose sign count went
// backwards may have been copied, so it can't sign in.
func (w *WebAuthn) FinishSignIn(_ Site, state, response []byte, find func(handle []byte) (Account, error)) (
	Account, Passkey, error,
) {
	var session webauthn.SessionData
	if err := json.Unmarshal(state, &session); err != nil {
		return Account{}, Passkey{}, ErrPasskeyRejected
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return Account{}, Passkey{}, ErrPasskeyRejected
	}
	var account Account
	handler := func(_, handle []byte) (webauthn.User, error) {
		a, err := find(handle)
		if err != nil {
			return nil, err
		}
		account = a
		return waUser{a}, nil
	}
	_, cred, err := w.wa.ValidatePasskeyLogin(handler, session, parsed)
	if err != nil || !cred.Flags.UserVerified || cred.Authenticator.CloneWarning {
		return Account{}, Passkey{}, ErrPasskeyRejected
	}
	return account, fromCredential(*cred), nil
}

func marshalCeremony(options any, session *webauthn.SessionData) (json.RawMessage, []byte, error) {
	opts, err := json.Marshal(options)
	if err != nil {
		return nil, nil, fmt.Errorf("accounts: encode passkey options failed (%v)", err)
	}
	state, err := json.Marshal(session)
	if err != nil {
		return nil, nil, fmt.Errorf("accounts: encode passkey ceremony failed (%v)", err)
	}
	return opts, state, nil
}

// waUser is an Account as go-webauthn sees it.
type waUser struct {
	a Account
}

func (u waUser) WebAuthnID() []byte          { return u.a.Handle }
func (u waUser) WebAuthnName() string        { return u.a.Name }
func (u waUser) WebAuthnDisplayName() string { return u.a.Name }

func (u waUser) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, len(u.a.Passkeys))
	for i, k := range u.a.Passkeys {
		transports := make([]protocol.AuthenticatorTransport, len(k.Transports))
		for j, t := range k.Transports {
			transports[j] = protocol.AuthenticatorTransport(t)
		}
		creds[i] = webauthn.Credential{
			ID:              k.ID,
			PublicKey:       k.PublicKey,
			AttestationType: k.AttestationType,
			Transport:       transports,
			Flags: webauthn.CredentialFlags{
				UserPresent: true, UserVerified: true, BackupEligible: k.BackupEligible, BackupState: k.BackupState,
			},
			Authenticator: webauthn.Authenticator{AAGUID: k.AAGUID, SignCount: k.SignCount},
		}
	}
	return creds
}

func fromCredential(c webauthn.Credential) Passkey {
	transports := make([]string, len(c.Transport))
	for i, t := range c.Transport {
		transports[i] = string(t)
	}
	return Passkey{
		ID:              c.ID,
		PublicKey:       c.PublicKey,
		AttestationType: c.AttestationType,
		Transports:      transports,
		AAGUID:          c.Authenticator.AAGUID,
		SignCount:       c.Authenticator.SignCount,
		BackupEligible:  c.Flags.BackupEligible,
		BackupState:     c.Flags.BackupState,
	}
}

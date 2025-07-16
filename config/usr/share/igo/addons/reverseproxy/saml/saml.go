package saml

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"net/url"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

var (
	SAMLConf = SAMLConfig{}
)

type SAMLConfig struct {
	RedirectParameter string
	IdpMetadataURL    string
	EntityID          string
	CookieName        string
	RootURL           string
	CertFile          string
	KeyFile           string
	Domain            string
	AuthnNameIDFormat string
}

func InitSAML() (*samlsp.Middleware, error) {
	keyPair, err := tls.LoadX509KeyPair(SAMLConf.CertFile, SAMLConf.KeyFile)
	if err != nil {
		panic(err)
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		panic(err)
	}

	idpMetadataURL, err := url.Parse(SAMLConf.IdpMetadataURL)
	if err != nil {
		panic(err)
	}
	idpMetadata, err := samlsp.FetchMetadata(context.Background(), http.DefaultClient,
		*idpMetadataURL)
	if err != nil {
		panic(err)
	}

	rootURL, err := url.Parse(SAMLConf.RootURL)
	if err != nil {
		panic(err)
	}

	opt := samlsp.Options{
		URL:               *rootURL,
		Key:               keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate:       keyPair.Leaf,
		IDPMetadata:       idpMetadata,
		AllowIDPInitiated: true,
		EntityID:          SAMLConf.EntityID,
		CookieName:        SAMLConf.CookieName,
	}
	if mw, err := samlsp.New(opt); err == nil {
		mw.ServiceProvider.AuthnNameIDFormat = saml.NameIDFormat(SAMLConf.AuthnNameIDFormat)
		mw.Session = DefaultSessionProvider(opt, SAMLConf.Domain)
		return mw, nil
	} else {
		log.Println("samlsp.New err ", err)
		return nil, err
	}
}

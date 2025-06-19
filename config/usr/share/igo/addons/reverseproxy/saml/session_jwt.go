package saml

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

var defaultJWTSigningMethod = jwt.SigningMethodRS256

const (
	defaultSessionMaxAge  = time.Hour * 8
	claimNameSessionIndex = "SessionIndex"
)

// JWTSessionCodec implements SessionCoded to encode and decode Sessions from
// the corresponding JWT.
type JWTSessionCodec struct {
	SigningMethod jwt.SigningMethod
	Audience      string
	Issuer        string
	MaxAge        time.Duration
	Key           crypto.Signer
}

type JWTUser struct {
	Domain string
	Name   string
	Email  string
}

var _ samlsp.SessionCodec = JWTSessionCodec{}

// New creates a Session from the SAML assertion.
//
// The returned Session is a JWTSessionClaims.
func (c JWTSessionCodec) New(assertion *saml.Assertion) (samlsp.Session, error) {
	now := saml.TimeNow()
	claims := JWTSessionClaims{}
	claims.SAMLSession = true
	claims.Audience = c.Audience
	claims.Issuer = c.Issuer
	claims.IssuedAt = now.Unix()
	claims.ExpiresAt = now.Add(c.MaxAge).Unix()
	claims.NotBefore = now.Unix()

	if sub := assertion.Subject; sub != nil {
		if nameID := sub.NameID; nameID != nil {
			claims.Subject = nameID.Value
		}
	}

	claims.Attributes = map[string][]string{}

	for _, attributeStatement := range assertion.AttributeStatements {
		for _, attr := range attributeStatement.Attributes {
			claimName := attr.FriendlyName
			if claimName == "" {
				claimName = attr.Name
			}
			// fmt.Println(claimName)
			// if strings.HasSuffix(claimName, "memberOf") {
			if strings.HasSuffix(claimName, "emailaddress") {
				claimName = "emailaddress"
				for _, value := range attr.Values {
					claims.Attributes[claimName] = append(claims.Attributes[claimName], value.Value)
				}
			}
		}
	}

	// add SessionIndex to claims Attributes
	for _, authnStatement := range assertion.AuthnStatements {
		claims.Attributes[claimNameSessionIndex] = append(claims.Attributes[claimNameSessionIndex],
			authnStatement.SessionIndex)
	}

	return claims, nil
}

// Encode returns a serialized version of the Session.
//
// The provided session must be a JWTSessionClaims, otherwise this
// function will panic.
func (c JWTSessionCodec) Encode(s samlsp.Session) (string, error) {
	claims := s.(JWTSessionClaims) // this will panic if you pass the wrong kind of session

	token := jwt.NewWithClaims(c.SigningMethod, claims)
	signedString, err := token.SignedString(c.Key)
	if err != nil {
		return "", err
	}

	return signedString, nil
}

// Decode parses the serialized session that may have been returned by Encode
// and returns a Session.
func (c JWTSessionCodec) Decode(signed string) (samlsp.Session, error) {
	parser := jwt.Parser{
		ValidMethods: []string{c.SigningMethod.Alg()},
	}
	claims := JWTSessionClaims{}
	_, err := parser.ParseWithClaims(signed, &claims, func(*jwt.Token) (interface{}, error) {
		return c.Key.Public(), nil
	})
	// TODO(ross): check for errors due to bad time and return ErrNoSession
	if err != nil {
		return nil, err
	}
	if !claims.VerifyAudience(c.Audience, true) {
		return nil, fmt.Errorf("expected audience %q, got %q", c.Audience, claims.Audience)
	}
	if !claims.VerifyIssuer(c.Issuer, true) {
		return nil, fmt.Errorf("expected issuer %q, got %q", c.Issuer, claims.Issuer)
	}
	if !claims.SAMLSession {
		return nil, errors.New("expected saml-session")
	}
	return claims, nil
}

// JWTSessionClaims represents the JWT claims in the encoded session
type JWTSessionClaims struct {
	jwt.StandardClaims
	Attributes  Attributes `json:"attr"`
	SAMLSession bool       `json:"saml-session"`
}

var _ samlsp.Session = JWTSessionClaims{}

// GetAttributes implements SessionWithAttributes. It returns the SAMl attributes.
func (c JWTSessionClaims) GetAttributes() Attributes {
	return c.Attributes
}

// Attributes is a map of attributes provided in the SAML assertion
type Attributes map[string][]string

// Get returns the first attribute named `key` or an empty string if
// no such attributes is present.
func (a Attributes) Get(key string) string {
	if a == nil {
		return ""
	}
	v := a[key]
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

func AttributeFromContext(ctx context.Context, name string) string {
	s := samlsp.SessionFromContext(ctx)
	if s == nil {
		return ""
	}
	sa, ok := s.(JWTSessionClaims)
	if !ok {
		return ""
	}
	fmt.Println(sa.StandardClaims.Subject)
	return sa.GetAttributes().Get(name)
}

func Pop(xs *[]string) string {
	if len(*xs) > 0 {
		x := (*xs)[len(*xs)-1]
		*xs = (*xs)[:len(*xs)-1]
		return x
	}
	return ""
}

func UserFromRequest(r *http.Request) JWTUser {
	domainAndName := ""
	s := samlsp.SessionFromContext(r.Context())
	if s == nil {
		domainAndName = ""
	}
	sa, ok := s.(JWTSessionClaims)
	if !ok {
		domainAndName = ""
	}
	domainAndName = sa.StandardClaims.Subject
	user := strings.Split(domainAndName, "\\")
	return JWTUser{
		Name:   Pop(&user),
		Domain: Pop(&user),
		Email:  AttributeFromContext(r.Context(), "emailaddress"),
	}

}

func DefaultSessionProvider(opts samlsp.Options, domain string) samlsp.CookieSessionProvider {
	cookieName := opts.CookieName

	return samlsp.CookieSessionProvider{
		Name:     cookieName,
		Domain:   domain,
		MaxAge:   defaultSessionMaxAge,
		HTTPOnly: true,
		Secure:   opts.URL.Scheme == "https",
		SameSite: opts.CookieSameSite,
		Codec: JWTSessionCodec{
			SigningMethod: defaultJWTSigningMethod,
			Audience:      opts.URL.String(),
			Issuer:        opts.URL.String(),
			MaxAge:        defaultSessionMaxAge,
			Key:           opts.Key,
		},
	}
}

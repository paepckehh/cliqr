# OVERVIEW 
[![Go Reference](https://pkg.go.dev/badge/paepcke.de/cliqr.svg)](https://pkg.go.dev/paepcke.de/cliqr) [![Go Report Card](https://goreportcard.com/badge/paepcke.de/cliqr)](https://goreportcard.com/report/paepcke.de/cliqr)

[paepche.de/cliqr](https://paepcke.de/cliqr/)

Display QR codes on your console to secure transfer locally:
- keys, secrets, access token
- (small-ec-based) certificates
- urls, uris
- wifi creds
- code fragments

# KEYPOINTS

- minimal code, static, dependency free 
- backend is a boiled-down fork of [skip2/go-qrcode](https://github.com/skip2/go-qrcode) (***ALL CREDIT GOES THERE!***)
- 100% pure go, stdlib only, use as app or api (see api.go)

# INSTALL

```
go install paepcke.de/cliqr/cmd/cliqr@latest
```

### DOWNLOAD (prebuild)

[github.com/paepckehh/cliqr/releases](https://github.com/paepckehh/cliqr/releases)

# SHOWTIME 

```Shell 

cliqr "mailto:potus@wh.gov"
[...]

cliqr "WIFI:S:$my-ssid;T:WPA;P:$my-password"
[...]

cat /etc/ssh/ssh-ed25519-key | cliqr
[...]

echo "MYSUPERSECRETPASSWORD" | cliqr
[...]

echo $BTC | cliqr
[...]

```

# DOCS

[pkg.go.dev/paepcke.de/cliqr](https://pkg.go.dev/paepcke.de/cliqr)

# CONTRIBUTION

Yes, Please! PRs Welcome! 

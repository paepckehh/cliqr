# OVERVIEW 

[paepche.de/cliqr](https://paepcke.de/cliqr)

Display QR codes on your console to secure transfer locally:  
 - keys, secrets, access token
 - (small-ec-based) certificates
 - urls, uris
 - wifi creds
 - code snippets

- boiled-down minmal static backend fork of [skip2/go-qrcode](https://github.com/skip2/go-qrcode) (***ALL CREDIT GOES THERE!***)
- small, fast, 100% external dependency free, use as app or api (see api.go)

# INSTALL

```
go install paepcke.de/cliqr/cmd/cliqr@latest
```

# SHOWTIME 

```Shell 

cliqr "MAILTO:potus@wh.gov"
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

# CONTRIBUTION

Yes, Please! PRs Welcome! 

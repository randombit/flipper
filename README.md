Flipper
========

Flipper is a caching server for Arch Linux packages. The idea is to
conserve bandwidth when there are multiple Arch machines on the same LAN.
There are several somewhat similar tools, but most rely on multicast.

It is most similar to using nginx as a
[reverse proxy cache](https://wiki.archlinux.org/index.php/Pacman/Tips_and_tricks#Dynamic_reverse_proxy_cache_using_nginx),
which is what I currently use for this. However the goal is to extend
flipper beyond this by adding features not really possible with nginx
setup:

- Use the [mirror health data](https://www.archlinux.org/mirrors/status/)
  so out of sync or incomplete mirrors are avoided.

- Nightly performance tests of mirrors (download a file, compute byte/sec,
  order by fastest)

- Handle mirrors with different directory structures.

- Automatic cache cleanout

- Caching of db files (if cached and a new request comes in, check if
  the upstream db has changed using size/date/Etag).

- Pre-caching packages (if the last N versions of something were
  installed, and a new version of same package shows up on the
  mirrors, download it in the middle of the night.)

Other todos:

- [ ] Use configuration file instead of command line flags
- [ ] Add a systemd unit file
- [ ] pkgbuild for AUR

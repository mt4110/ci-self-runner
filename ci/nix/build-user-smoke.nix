# Container-only smoke: /bin/sh and /usr/bin utilities intentionally come from
# the Ubuntu job image. Run with --impure; this is not a portable package build.
derivation {
  name = "ci-self-nix-build-user-smoke";
  system = builtins.currentSystem;
  builder = "/bin/sh";
  args = [ "-eu" "-c" ''
    uid=$(/usr/bin/id -u)
    test "$uid" -ne 0
    /bin/mkdir protected
    /bin/chmod 555 protected
    if /bin/sh -c 'echo unexpected > protected/new-file' 2>/dev/null; then
      echo 'builder bypassed directory permissions' >&2
      exit 1
    fi
    /bin/chmod 755 protected
    printf 'builder_uid=%s; permission denial verified\n' "$uid" > "$out"
    /bin/cat "$out"
  '' ];
}

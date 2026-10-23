# Local VM kernel/build smoke, using Ubuntu's static Bash as the only input.
# Invoke with --impure to allow this explicitly named local system file.
let
  shell = builtins.path { path = /bin/bash-static; name = "vm-smoke-bash"; };
in
derivation {
  name = "ci-self-x64-nix-build-smoke";
  system = "x86_64-linux";
  builder = shell;
  args = [ "-c" ''printf '%s\n' 'x86_64 Nix builder completed' > "$out"'' ];
}

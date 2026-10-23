{
  description = "Development environment for ci-self-runner";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs = { nixpkgs, ... }:
    let
      systems = [ "aarch64-darwin" "x86_64-darwin" "aarch64-linux" "x86_64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
          tools = [ pkgs.go_1_25 pkgs.bash pkgs.git pkgs.act pkgs.docker-client ];
          test = pkgs.writeShellApplication {
            name = "ci-self-test";
            runtimeInputs = tools;
            text = ''
              # Avoid a host Go installation or automatic toolchain downloads.
              unset GOROOT GOTOOLDIR GOENV
              export GOTOOLCHAIN=local
              unformatted="$(gofmt -l cmd ops)"
              if [[ -n "$unformatted" ]]; then
                printf 'ERROR: unformatted Go files:\n%s\n' "$unformatted" >&2
                exit 1
              fi
              go vet ./...
              go test ./...
            '';
          };
        in
        {
          default = pkgs.mkShellNoCC {
            packages = tools ++ [ test ];
            GOTOOLCHAIN = "local";
            shellHook = ''
              unset GOROOT GOTOOLDIR GOENV
            '';
          };
          vm = pkgs.mkShellNoCC {
            packages = tools ++ [ test pkgs.qemu_full ];
            GOTOOLCHAIN = "local";
            shellHook = ''
              unset GOROOT GOTOOLDIR GOENV
            '';
          };
        });
    };
}

ARG BASE_IMAGE
ARG NIX_IMAGE
FROM ${NIX_IMAGE} AS nix
# Keep the runtime closure and its database; omit the bundled Nixpkgs source
# and unrelated packages from the official image. Signature checking stays on.
RUN NIX_PACKAGE="$(dirname "$(dirname "$(readlink -f /nix/var/nix/profiles/default/bin/nix)")")" \
    && nix --extra-experimental-features nix-command copy --from https://cache.nixos.org --to /runtime "$NIX_PACKAGE" \
    && mkdir -p /runtime/nix/var/nix/profiles \
    && ln -s "$NIX_PACKAGE" /runtime/nix/var/nix/profiles/default
FROM ${BASE_IMAGE}
USER root
COPY --from=nix /runtime/nix /nix
COPY --from=nix /etc/nix/nix.conf /etc/nix/nix.conf
# Single-user Nix inside the disposable job container. No daemon or privileged mode.
RUN printf '\nbuild-users-group =\nexperimental-features = nix-command flakes\n' >> /etc/nix/nix.conf
ENV PATH="${PATH}:/nix/var/nix/profiles/default/bin"
ENV NIX_SSL_CERT_FILE="/etc/ssl/certs/ca-certificates.crt"
RUN nix --version

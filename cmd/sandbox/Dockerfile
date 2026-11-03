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
# Keep workflow setup as root, but run Nix builders as unprivileged users.
# Permission tests must not silently succeed through root's DAC override.
# No privileged container or extra daemon is required.
RUN groupadd --system nixbld \
    && for n in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16; do \
         useradd --system --gid nixbld --groups nixbld --no-create-home \
           --home-dir /var/empty --shell /usr/sbin/nologin "nixbld$n"; \
       done \
    && chgrp nixbld /nix/store && chmod 1775 /nix/store \
    && printf '\nbuild-users-group = nixbld\nexperimental-features = nix-command flakes\n' >> /etc/nix/nix.conf
ENV PATH="${PATH}:/nix/var/nix/profiles/default/bin"
ENV NIX_SSL_CERT_FILE="/etc/ssl/certs/ca-certificates.crt"
RUN nix --version

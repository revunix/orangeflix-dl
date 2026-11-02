{
  description = "Hilfswerkzeug zur lokalen Archivierung der Orangeflix-Folgen";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      version = "1.4.0";
      repo = "revunix/orangeflix-dl";
      systems = {
        x86_64-linux = { os = "linux"; arch = "amd64"; hash = "sha256-IfIZm8cI6xUO+ixMfmO8ITZ9jssU4f+gxd3z4CpeVO4="; };
        aarch64-linux = { os = "linux"; arch = "arm64"; hash = "sha256-I4hnIVrhSwza7Azqzd/jbLjSKW8hmJjpCcAhA3SjYgE="; };
        x86_64-darwin = { os = "darwin"; arch = "amd64"; hash = "sha256-9/pjhI09oi34DGw4MTpFql0NUiW29QM3DOFCvh21VL0="; };
        aarch64-darwin = { os = "darwin"; arch = "arm64"; hash = "sha256-ZPpjAxMW/prwkW8kNxT88Np8XDgWOT5XCSynaI2KkK4="; };
      };
      each = nixpkgs.lib.genAttrs (builtins.attrNames systems);
    in
    {
      packages = each (system:
        let
          pkgs = import nixpkgs { inherit system; };
          spec = systems.${system};
          archive = "orangeflix-dl_v${version}_${spec.os}_${spec.arch}.tar.gz";
        in
        {
          default = pkgs.stdenvNoCC.mkDerivation {
            pname = "orangeflix-dl";
            inherit version;
            src = pkgs.fetchurl {
              url = "https://github.com/${repo}/releases/download/v${version}/${archive}";
              hash = spec.hash;
            };
            nativeBuildInputs = [ pkgs.installShellFiles ];
            dontUnpack = true;
            installPhase = ''
              runHook preInstall
              tar -xzf $src
              install -Dm755 orangeflix-dl $out/bin/orangeflix-dl
              installShellCompletion --cmd orangeflix-dl \
                --bash <($out/bin/orangeflix-dl --completions bash) \
                --zsh <($out/bin/orangeflix-dl --completions zsh) \
                --fish <($out/bin/orangeflix-dl --completions fish)
              $out/bin/orangeflix-dl --man > orangeflix-dl.1
              installManPage orangeflix-dl.1
              runHook postInstall
            '';
          };
        });
    };
}

# Copyright 2024 Ross Light
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

{
  description = "Asana Markdown Export";

  inputs = {
    nixpkgs.url = "nixpkgs";
    flake-utils.url = "flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        myTasksRuntimeInputs = [
          pkgs.curl
          pkgs.jq
        ];
      in
      {
        packages.default = pkgs.symlinkJoin {
          name = "asana-md-export";
          paths = [
            self.packages.${system}.asana-to-md
            self.packages.${system}.asana-my-tasks
          ];
        };

        packages.asana-to-md =
          let
            inherit (pkgs) nix-gitignore;

            buildGoModule = pkgs.buildGo121Module;

            root = ./.;
            patterns = nix-gitignore.withGitignoreFile extraIgnores root;
            extraIgnores = [
              "/nix/"
              "*.nix"
              "flake.lock"
              "/.github/"
              ".vscode/"
              "result"
              "result-*"
              "*.sh"
            ];
          in
            buildGoModule {
              name = "asana-to-md";

              src = builtins.path {
                name = "asana-md-export";
                path = root;
                filter = nix-gitignore.gitignoreFilterPure (_: _: true) patterns root;
              };

              vendorHash = "sha256-jgdFossNwOyr9FkFM72xZtzIDIj3G5kqqzLtMWaRSFo=";

              subPackages = [ "./cmd/asana-to-md" ];
              ldflags = [ "-s" "-w" ];

              meta = {
                homepage = "https://github.com/zombiezen/asana-md-export";
                license = pkgs.lib.licenses.asl20;
              };
            };

        packages.asana-my-tasks = pkgs.writeShellApplication {
          name = "asana-my-tasks";

          runtimeInputs = myTasksRuntimeInputs;

          text = builtins.readFile ./asana-my-tasks.sh;

          meta = {
            homepage = "https://github.com/zombiezen/asana-md-export";
            license = pkgs.lib.licenses.asl20;
          };
        };

        devShells.default = pkgs.mkShell {
          packages = myTasksRuntimeInputs;

          inputsFrom = [ self.packages.${system}.asana-to-md ];
        };
      }
    );
}

{ lib, buildGoModule }:
buildGoModule {
  pname = "oddc";
  version = "0.1.0";
  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      ./go.mod
      (lib.fileset.fileFilter (file: file.hasExt "go") ./.)
      ./catalog
      ./evidence
      ./schemas
    ];
  };
  vendorHash = null;
  subPackages = [ "cmd/oddc" ];
  # The registry tests read the catalog from the source root.
  doCheck = true;
  meta = {
    description = "Validate, resolve and explain ODDC hardware entities";
    license = lib.licenses.asl20;
    mainProgram = "oddc";
  };
}

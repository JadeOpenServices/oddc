{
  lib,
  buildGoModule,
  git,
}:
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
      # The update tests take the oddc input from the README.
      ./README.md
    ];
  };
  vendorHash = null;
  subPackages = [ "cmd/oddc" ];
  # The tests live under tests/ and read the catalog from the source root;
  # the default check would only test subPackages.
  doCheck = true;
  checkPhase = ''
    runHook preCheck
    go test ./...
    runHook postCheck
  '';
  # validate --since compares revisions with git.
  nativeCheckInputs = [ git ];
  meta = {
    description = "Validate, resolve and explain ODDC hardware entities";
    license = lib.licenses.asl20;
    mainProgram = "oddc";
  };
}

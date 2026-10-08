# SPDX-License-Identifier: GPL-3.0-or-later

# root holds an ODDC catalog: this repository, or an answer `oddc fetch`
# wrote. Either way entities live at catalog/entities/<id>.json and
# evidence at evidence/<model id>/.
{
  lib,
  root ? ../.,
}:

let
  entityRoot = root + "/catalog/entities";

  walk =
    dir:
    let
      entries = builtins.readDir dir;
    in
    lib.concatMap (
      name:
      let
        kind = entries.${name};
        path = dir + "/${name}";
      in
      if kind == "directory" then
        walk path
      else if kind == "regular" && lib.hasSuffix ".json" name then
        [ path ]
      else
        [ ]
    ) (builtins.attrNames entries);

  files = map (path: {
    inherit path;
    relative = builtins.unsafeDiscardStringContext (
      lib.removePrefix "${toString entityRoot}/" (toString path)
    );
    document = builtins.fromJSON (builtins.readFile path);
  }) (walk entityRoot);

  entities = builtins.listToAttrs (
    map (file: {
      name = file.document.metadata.id;
      value = file.document;
    }) files
  );

  # Entity ID -> { path, relative } of its canonical file.
  entityFiles = builtins.listToAttrs (
    map (file: {
      name = file.document.metadata.id;
      value = { inherit (file) path relative; };
    }) files
  );

  refsOf =
    value:
    if builtins.isAttrs value then
      (if value ? ref then [ value.ref ] else [ ])
      ++ lib.concatMap refsOf (builtins.attrValues (builtins.removeAttrs value [ "ref" ]))
    else
      [ ];

  # Every entity ID one entity depends on, itself included.
  closure =
    id:
    let
      step =
        seen: pending:
        if pending == [ ] then
          seen
        else
          let
            next = builtins.head pending;
            rest = builtins.tail pending;
          in
          if builtins.elem next seen then
            step seen rest
          else
            step (seen ++ [ next ]) (rest ++ refsOf (builtins.getAttr next entities).data);
    in
    step [ ] [ id ];

  evidenceRoot = root + "/evidence";

  # Evidence records about one model, at evidence/<id>/: list of
  # { path, relative }.
  evidenceFor =
    id:
    let
      dir = evidenceRoot + "/${id}";
    in
    if !builtins.pathExists dir then
      [ ]
    else
      map (path: {
        inherit path;
        relative = builtins.unsafeDiscardStringContext (
          lib.removePrefix "${toString evidenceRoot}/" (toString path)
        );
      }) (walk dir);

  # The revision an answer records, else null.
  revision =
    let
      file = root + "/revision";
    in
    if builtins.pathExists file then lib.trim (builtins.readFile file) else null;

  resolveValue =
    active: value:
    if builtins.isAttrs value then
      if value ? ref then
        let
          ref = value.ref;

          target =
            if builtins.elem ref active then
              throw "ODDC entity reference cycle through ${ref}"
            else if builtins.hasAttr ref entities then
              builtins.getAttr ref entities
            else
              throw "ODDC entity ${builtins.head active} references missing entity ${ref}";

          base = resolveValue (active ++ [ ref ]) target.data;

          local = resolveValue active (builtins.removeAttrs value [ "ref" ]);

          baseWithIdentity = base // {
            id = target.metadata.id;
            name = target.metadata.name;
            kind = target.kind;
          };
        in
        lib.recursiveUpdate baseWithIdentity local
      else
        lib.mapAttrs (_: child: resolveValue active child) value
    else
      value;

  resolveEntity =
    id:
    let
      entity =
        if builtins.hasAttr id entities then
          builtins.getAttr id entities
        else
          throw "Unknown ODDC entity: ${id}";
    in
    {
      inherit (entity) kind;

      id = entity.metadata.id;
      name = entity.metadata.name;
      resolved = resolveValue [ id ] entity.data;
    };

  modelIds = builtins.filter (id: lib.hasPrefix "model/" id) (builtins.attrNames entities);
in
{
  inherit
    closure
    entities
    entityFiles
    evidenceFor
    modelIds
    resolveEntity
    revision
    ;
}

#!/usr/bin/env python3
"""Recover exact tracked AutoMesh package files; never run the uploaded EXE."""
import ast, hashlib, json, re, struct, subprocess, zipfile, stat, shutil
from pathlib import Path, PurePosixPath

ROOT=Path.cwd()
ZIP="AutoMesh_R8_1_BatchHotspot_20261110_Full.zip"
BLOB="a8090839f29eb8b7e612438a9256e61c4535a650"
def digest(data):return hashlib.sha256(data).hexdigest()
def run(*args):return subprocess.check_output(args,cwd=ROOT)
def clean_name(info):
    s=info.filename
    if not info.flag_bits & 0x800:
        try:
            candidate=s.encode("cp437").decode("gb18030")
            if any("\u4e00"<=c<="\u9fff" for c in candidate):s=candidate
        except (UnicodeError,ValueError):pass
    s=s.replace("\\","/")
    p=PurePosixPath(s)
    if p.is_absolute() or not p.parts or any(x in (".","..",".git") or ":" in x or "\x00" in x for x in p.parts):
        raise ValueError("unsafe archive member: "+s)
    return p
def put(path,data):
    p=ROOT/path;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data)
def txt(path,s):put(path,s.encode("utf-8"))
def git_file(path):return run("git","show","HEAD:"+path)
def relpath(p,root):return p.relative_to(root).as_posix()
def main():
    archive=(ROOT/ZIP).read_bytes()
    if hashlib.sha1(b"blob "+str(len(archive)).encode()+b"\0"+archive).hexdigest()!=BLOB or len(archive)!=5850656:
        raise SystemExit("Uploaded package does not match pinned Git blob")
    entries={}; skipped=[]; names={}
    with zipfile.ZipFile(ROOT/ZIP) as z:
        infos=z.infolist()
        if len(infos)>10000 or sum(x.file_size for x in infos)>500_000_000:raise ValueError("archive too large")
        if z.testzip() is not None:raise ValueError("archive CRC failure")
        for x in infos:
            p=clean_name(x)
            if stat.S_ISLNK(x.external_attr>>16) or x.flag_bits&1:raise ValueError("symlink/encrypted archive entry: "+x.filename)
            if x.is_dir():continue
            if any(c in ("__pycache__", ".pytest_cache", ".venv", "__MACOSX") for c in p.parts) or p.suffix in (".pyc",".pyo") or p.name in (".DS_Store","Thumbs.db"):
                skipped.append({"archive_path":x.filename,"reason":"generated cache or OS metadata"});continue
            if p in entries:raise ValueError("duplicate normalized path: "+str(p))
            entries[p]=z.read(x);names[p]=x.filename
    roots=[p.parent for p in entries if p.name=="go.mod" and p.parent/"main_windows.go" in entries and p.parent/"assets/meshing_worker.py" in entries]
    if len(roots)!=1:raise ValueError("expected unique source root, found "+str(roots)+"; top members "+str(list(map(str,entries))[:40]))
    root=roots[0]
    exes=[(p,b) for p,b in entries.items() if p.name.lower()=="automesh.exe"]
    if not exes or len({digest(b) for p,b in exes})!=1:raise ValueError("missing or ambiguous AutoMesh.exe")
    ep,exe=exes[0]
    if len(exe)<64 or exe[:2]!=b"MZ":raise ValueError("invalid EXE")
    pos=struct.unpack_from("<I",exe,0x3c)[0]
    if exe[pos:pos+4]!=b"PE\0\0" or struct.unpack_from("<H",exe,pos+4)[0]!=0x8664:raise ValueError("expected Windows x64 PE")
    common=PurePosixPath(".")
    if root.parts:common=PurePosixPath(root.parts[0])
    recovered={}; provenance={}; casefold=set()
    def record(target,p,data):
        if target in recovered or target.casefold() in casefold:raise ValueError("duplicate target "+target)
        casefold.add(target.casefold());recovered[target]=data;provenance[target]=names[p]
    record("app/AutoMesh.exe",ep,exe)
    for p,data in entries.items():
        if p.name.lower()=="automesh.exe":
            if p!=ep:skipped.append({"archive_path":names[p],"reason":"byte-identical executable duplicate"})
            continue
        if any(c in (".github",".git") for c in p.parts): # retain as metadata, never activate archive workflows
            target="package_extras/"+p.as_posix()
        elif p.is_relative_to(root):
            target="source/"+relpath(p,root)
        else:
            target="package_extras/"+(relpath(p,common) if p.is_relative_to(common) else p.as_posix())
        record(target,p,data)
    core=entries[root/"core.go"].decode("utf-8-sig",errors="replace") if root/"core.go" in entries else ""
    main_src=entries[root/"main_windows.go"].decode("utf-8-sig",errors="replace")
    version_lines=[l.strip() for l in (core+"\n"+main_src).splitlines() if re.search(r"(?:appVersion|buildVersion|versionLabel|releaseVersion|AutoMesh\s+R8)",l,re.I)][:25]
    source_old={p for p in run("git","ls-files","-z").decode().split("\0") if p.startswith("source/") or p=="app/AutoMesh.exe"}
    source_new=set(recovered)
    common_paths=source_old&source_new
    delta={"added":sorted(source_new-source_old),"removed":sorted(source_old-source_new),
           "changed":sorted(p for p in common_paths if digest(git_file(p))!=digest(recovered[p])),
           "unchanged":sorted(p for p in common_paths if digest(git_file(p))==digest(recovered[p]))}
    # Static checks only: parse source with the standard library; archive contents unchanged.
    py_ok=[];py_errors={}
    for p,data in recovered.items():
        if p.endswith(".py"):
            try:ast.parse(data,filename=p);py_ok.append(p)
            except (SyntaxError,ValueError) as e:py_errors[p]=str(e)
    # Extract a PNG from the original application icon if one is embedded.
    icon_png=None
    for p in ("source/AutoMesh.ico","source/icon.ico"):
        data=recovered.get(p,b"")
        if len(data)<6 or data[:4]!=b"\0\0\1\0":continue
        n=struct.unpack_from("<H",data,4)[0]
        choices=[]
        for k in range(n):
            j=6+16*k
            if j+16>len(data):break
            w,h,_,_,_,_,length,offset=struct.unpack_from("<BBBBHHII",data,j)
            b=data[offset:offset+length]
            if b.startswith(b"\x89PNG\r\n\x1a\n"):choices.append(((w or 256)*(h or 256),b))
        if choices:icon_png=max(choices,key=lambda x:x[0])[1];break
    # Remove all previous tracked files, except exact new ZIP (recreated below).
    tracked=run("git","ls-files","-z").decode("utf-8").split("\0")
    for name in tracked:
        if name:
            p=ROOT/name
            if p.is_file() or p.is_symlink():p.unlink()
    for p,data in recovered.items():put(p,data)
    put(ZIP,archive)
    if icon_png:put("docs/media/icon.png",icon_png)
    inventory={"package":ZIP,"uploaded_date":"2026-11-22","archive_sha256":digest(archive),"archive_git_blob":BLOB,
       "archive_size_bytes":len(archive),"source_root":str(root),"executable_sha256":digest(exe),"executable_size_bytes":len(exe),
       "recovered":[{"path":p,"archive_path":provenance[p],"sha256":digest(b),"size":len(b)} for p,b in sorted(recovered.items())],
       "skipped":skipped,"comparison_to_previous_tree":delta,"version_source_lines":version_lines,
       "checks":{"zip_crc":"pass","git_blob":"pass","windows_x64_pe":"pass","python_ast_passed":len(py_ok),"python_ast_errors":py_errors},
       "scope":"Exact package recovery; no Windows/ANSYS executable run or simulation acceptance. Archive paths normalized only; generated caches and duplicate EXE omitted."}
    txt("docs/release_inventory.json",json.dumps(inventory,ensure_ascii=False,indent=2)+"\n")
    txt("SHA256SUMS.txt",digest(archive)+"  "+ZIP+"\n"+"".join(digest(b)+"  "+p+"\n" for p,b in sorted(recovered.items())))
    txt(".gitignore","__pycache__/\n*.py[cod]\n.pytest_cache/\n.venv/\n")
    txt(".gitattributes","* -text\n")
    title="R8.1 · BatchHotspot"
    image='<img src="docs/media/icon.png" width="96" alt="AutoMesh">\n\n' if icon_png else ""
    readme=f'''<div align="center">

{image}<h1>AutoMesh</h1>

<h3>Better meshes. Better engineering decisions.</h3>

<p><strong>让每一次网格优化，都有据可循。</strong></p>

<p>面向 ANSYS Meshing 的网格质量诊断与局部优化工作台。<br>
连接已有工程，识别局部缺陷，将重复的网格检查与优化操作整合为可追溯的工作流程。</p>

<a href="releases/latest"><img alt="Download for Windows" src="https://img.shields.io/badge/Download%20for%20Windows-0560CC?style=for-the-badge&logo=windows&logoColor=white"></a>
<a href="{ZIP}"><img alt="Full package" src="https://img.shields.io/badge/Full%20package-24292F?style=for-the-badge"></a>

[快速开始](docs/GETTING_STARTED.md) · [源码](source/) · [完整性核验](docs/release_inventory.json)

<sub>{title} &nbsp; / &nbsp; Windows x64 &nbsp; / &nbsp; ANSYS Workbench &amp; Meshing</sub>

</div>

## Make mesh optimization measurable.

AutoMesh 将 ANSYS Meshing 的质量检查、异常单元标记、局部尺寸控制与优化前后对比组织在同一原生桌面工作台。聚焦真实质量数据、局部缺陷和可审核的 Sizing 方案。

## What you can do

| 质量诊断 · Inspect | 局部优化 · Refine |
|---|---|
| 从实际网格读取质量指标，定位异常单元。 | 通过命名几何或缺陷区域制定局部尺寸控制。 |
| **质量对比 · Compare** | **工程连接 · Connect** |
| 记录质量数据与优化历史，为下一轮调整提供依据。 | 复用当前 Workbench 和 Meshing 工程，减少重复操作。 |

## Get started

1. 下载 [AutoMesh.exe](app/AutoMesh.exe) 或 [完整压缩包]({ZIP})。
2. 打开已有 ANSYS Workbench / Meshing 工程，运行软件。
3. 在连接工程后读取真实网格质量、生成局部优化方案并按界面提示审核发布。

[完整使用指南 →](docs/GETTING_STARTED.md)

## Project resources

[Windows application](app/) · [Go & Python source](source/) · [Full package]({ZIP}) · [SHA-256](SHA256SUMS.txt)

<details>
<summary><strong>Release integrity &amp; engineering scope</strong></summary>

本次以 2026-11-22 上传的 {ZIP} 为唯一应用来源。已原样恢复 {len(recovered)} 个项目文件（含 EXE），自检及说明见 [发布核验清单](docs/release_inventory.json)。历史版本不混入当前应用目录。静态校验不代表 Windows 与 ANSYS 实机验收。

</details>
'''
    readme=readme.replace('href="releases/latest"','href="app/AutoMesh.exe"')
    txt("README.md",readme)
    txt("docs/GETTING_STARTED.md",f"""# AutoMesh · {title}

## 下载

- [独立 Windows EXE](../app/AutoMesh.exe)
- [本次上传的完整压缩包](../{ZIP})

完整压缩包保留原始字节；源码和构建资源位于 [source/](../source/)。

## 工作流程

打开已有 Workbench 和目标 Meshing 工程，运行 AutoMesh，连接并读取质量数据。使用局部细化和质量诊断功能提出调整方案，审核发布前核验作用域。实际按钮及配置以当前程序为准。

升级时注意保留本地任务历史及配置，不要为升级删除整个数据目录。

本次同步仅采用最新上传包，未在 Windows / ANSYS 中执行程序。见 [完整核验](release_inventory.json)。
""")
    for p,b in recovered.items():
        assert digest((ROOT/p).read_bytes())==digest(b),p
    assert digest((ROOT/ZIP).read_bytes())==digest(archive)
    print(json.dumps({"root":str(root),"files":len(recovered),"exe_size":len(exe),"exe_sha256":digest(exe),"zip_sha256":digest(archive),"changes":{k:len(v) for k,v in delta.items()},"version":version_lines,"python_errors":py_errors,"icon_png":bool(icon_png),"skipped":len(skipped)},ensure_ascii=False))
if __name__=="__main__":main()

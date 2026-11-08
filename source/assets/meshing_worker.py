# -*- coding: utf-8 -*-
"""IronPython 2.7 in ANSYS Meshing 2024 R2; call via Run_In_Existing_Meshing.py.

No UI automation, no application launch, no geometry editing, no Fluent.
Uses previously successful ACT calls; version-specific properties are guarded.
"""
import json
import os
import hashlib
import traceback
import time
import math
import tempfile

execfile(os.path.join(os.path.dirname(JOB_FILE), "geometry_rules.py"))
execfile(os.path.join(os.path.dirname(JOB_FILE), "warning_refinement.py"))


def utext(value):
    try:
        return unicode(value)
    except NameError:
        return str(value)


def write(path, data):
    """Publish complete JSON receipts using a native Windows replace.

    R8.0.1 still depended on System.IO.File.Move/Replace overload dispatch in
    IronPython. On some ACT hosts those calls return an opaque tuple such as
    ('unknown', '\x00', 0, 1, ''), including for a brand-new job receipt.
    Prefer Win32 MoveFileExW (Unicode paths, replace-existing + write-through).
    If ctypes is unavailable in a particular ACT build, use a recoverable
    same-directory rename sequence. Never delete the prior receipt first.
    """
    directory = os.path.dirname(os.path.abspath(path))
    text = json.dumps(data, ensure_ascii=True, indent=2).encode('utf-8')
    fd, temp = tempfile.mkstemp(prefix='.amwrite_', suffix='.tmp', dir=directory)
    try:
        os.close(fd)
        with open(temp, 'wb') as h:
            h.write(text)
            h.flush()
            if hasattr(os, 'fsync'):
                try: os.fsync(h.fileno())
                except (OSError, IOError, AttributeError): pass

        if os.name == 'nt':
            native_error = None
            # MoveFileExW is a Windows API rather than a .NET overload. It
            # returns a real Win32 error code when a target is locked/denied.
            for attempt in range(6):
                try:
                    import ctypes
                    try:
                        kernel32 = ctypes.WinDLL('kernel32', use_last_error=True)
                    except TypeError:
                        kernel32 = ctypes.WinDLL('kernel32')
                    move = kernel32.MoveFileExW
                    try:
                        move.argtypes = [ctypes.c_wchar_p, ctypes.c_wchar_p, ctypes.c_uint32]
                        move.restype = ctypes.c_int
                    except Exception:
                        pass
                    flags = 0x00000008  # MOVEFILE_WRITE_THROUGH
                    if os.path.exists(path):
                        flags |= 0x00000001  # MOVEFILE_REPLACE_EXISTING
                    ok = move(ctypes.c_wchar_p(temp), ctypes.c_wchar_p(path), flags)
                    if ok:
                        return
                    try: code = int(ctypes.get_last_error())
                    except Exception:
                        try: code = int(kernel32.GetLastError())
                        except Exception: code = -1
                    try: message = ctypes.FormatError(code)
                    except Exception: message = 'Win32 MoveFileExW failed'
                    native_error = 'MoveFileExW winerror=%s: %s' % (code, utext(message).strip())
                except Exception as exc:
                    native_error = 'MoveFileExW unavailable/failed: %s' % utext(exc)
                    # If ctypes itself is unavailable, use the safe rename path.
                    break
                time.sleep(0.05 * (attempt + 1))

            # Recoverable fallback for IronPython builds without usable ctypes
            # or for a transient native file-sharing error. Keep the old file
            # until the fully written temp file is ready to take its place.
            backup = temp + '.previous'
            fallback_errors = []
            for attempt in range(6):
                moved_old = False
                try:
                    if os.path.exists(path):
                        os.rename(path, backup)
                        moved_old = True
                    try:
                        os.rename(temp, path)
                    except Exception:
                        if moved_old and os.path.exists(backup) and not os.path.exists(path):
                            os.rename(backup, path)
                        raise
                    if moved_old and os.path.exists(backup):
                        try: os.remove(backup)
                        except Exception: pass
                    return
                except Exception as exc:
                    fallback_errors.append(utext(exc))
                    if moved_old and os.path.exists(backup) and not os.path.exists(path):
                        try: os.rename(backup, path)
                        except Exception as restore_exc:
                            fallback_errors.append('restore failed: ' + utext(restore_exc))
                    time.sleep(0.05 * (attempt + 1))
            raise IOError('JSON receipt replace failed; %s; rename fallback: %s' %
                          (native_error or 'native API not available', ' | '.join(fallback_errors[-3:])))
        else:
            replace = getattr(os, 'replace', None)
            if replace is not None:
                replace(temp, path)
            else:
                os.rename(temp, path)
            return
    finally:
        if os.path.isfile(temp):
            try: os.remove(temp)
            except Exception: pass


def read(path):
    with open(path, 'r') as h:
        return json.load(h)


def meshing_process_identity():
    """Identify the ACT host, not Workbench or the AutoMesh GUI.

    The startup timestamp prevents a recycled Windows PID from being mistaken
    for the same Meshing process. No mutation or extra process is started.
    """
    try:
        from System.Diagnostics import Process
        proc = Process.GetCurrentProcess()
        return {'process_pid': int(proc.Id),
                'process_started_filetime': str(proc.StartTime.ToFileTimeUtc())}
    except Exception:
        return {}


def emit_progress(percent, stage, detail):
    """Best-effort per-job heartbeat. Percent is only for measurable stages.

    None means ANSYS is still working but does not expose an exact percentage.
    Progress errors must never stop or change the user's Meshing job.
    """
    try:
        current=globals().get('job', {})
        jobid=utext(current.get('job_id',''))
        if len(jobid)!=32:return
        payload={'job_id':jobid,'stage':utext(stage),'detail':utext(detail),
                 'percent':(None if percent is None else max(0,min(100,int(percent)))),
                 'timestamp':time.time()}
        payload.update(meshing_process_identity())
        filename=os.path.join(os.path.dirname(JOB_FILE),'job_progress_'+jobid+'.json')
        write(filename,payload)
    except Exception:
        pass



class AutoMeshCancelled(Exception):
    pass


def cancellation_checkpoint():
    """Check the request only between safe ACT operations; native mesh build
    cannot be forcibly interrupted here. Never remove a task lease."""
    current=globals().get('job',{})
    jid=utext(current.get('job_id',''))
    if len(jid)!=32:return
    filename=os.path.join(os.path.dirname(JOB_FILE),'cancel_'+jid+'.json')
    if not os.path.isfile(filename):return
    try:
        req=read(filename)
    except Exception:
        return
    if utext(req.get('job_id',''))==jid:
        raise AutoMeshCancelled('Operator cancellation acknowledged at an ACT safe checkpoint')


def resolve_current_model(model, extapi):
    """Read-only ACT readiness gate before reading any Model.*.Children.

    R4 assumed Model.Mesh and Model.NamedSelections could never be None.
    In a Workbench-attached or incompletely loaded Meshing editor either root
    can be unavailable. Never report that situation as an empty named list.
    Resolve only objects from the active ExtAPI project; never start/open any
    system or create objects here.
    """
    candidates=[('current Model', model)]
    project_model=None
    try:
        project_model=extapi.DataModel.Project.Model
    except Exception:
        pass
    if project_model is not None and project_model is not model:
        candidates.append(('ExtAPI current project Model',project_model))
    reports=[]
    for label,candidate in candidates:
        if candidate is None:
            reports.append(label+': Model unavailable')
            continue
        try:
            mesh=getattr(candidate,'Mesh',None)
        except Exception:
            mesh=None
        try:
            named=getattr(candidate,'NamedSelections',None)
        except Exception:
            named=None
        try:
            mesh_children=(None if mesh is None else getattr(mesh,'Children',None))
        except Exception:
            mesh_children=None
        try:
            named_children=(None if named is None else getattr(named,'Children',None))
        except Exception:
            named_children=None
        if mesh is not None and named is not None and mesh_children is not None and named_children is not None:
            return candidate
        reports.append('%s: Mesh=%s, Mesh.Children=%s, NamedSelections=%s, NamedSelections.Children=%s' %
                       (label,mesh is not None,mesh_children is not None,
                        named is not None,named_children is not None))
    raise RuntimeError(
        u'当前 Meshing 模型树尚未就绪，无法安全读取命名组。请检查当前打开的 Meshing 是否加载完毕、Workbench 选择的系统是否正确，再点击「刷新命名」。'
        u' 未清除或修改网格、命名组、Sizing；上次扫描结果未被覆盖。\nACT roots: '+utext('; '.join(reports)))


def live_control_inventory():
    """Read the current Meshing Sizing tree and AutoMesh QR coordinate systems.

    Refreshes only live control metadata; geometry remains the cached read-only
    snapshot after quick_geometry_identity has proved it is unchanged.
    """
    result = {'controls': [], 'controls_complete': False,
              'warning_coordinate_systems': [],
              'warning_coordinate_systems_complete': False}
    for c in Model.Mesh.Children:
        item = {'name': utext(c.Name), 'category': utext(c.DataModelObjectCategory)}
        try: item['object_id'] = int(c.ObjectId)
        except Exception: pass
        try: item['ids'] = [int(i) for i in c.Location.Ids]
        except Exception: pass
        try: item['mm'] = utext(c.ElementSize)
        except Exception: pass
        if item['name'].startswith('AutoMesh_QR_'):
            try:
                item['is_sizing'] = bool(_is_sizing(c))
                item['type'] = utext(c.Type)
                item['suppressed'] = bool(_is_control_suppressed(c))
                item['body_ids'] = sorted(int(v) for v in _effective_sizing_ids(c))
                item['size_mm'] = float(_read_control_mm(c))
                item['radius_mm'] = float(_length_mm(c.SphereRadius))
                cs = c.SphereCenter
                item['coordinate_system_name'] = utext(cs.Name)
                item['coordinate_system_object_id'] = int(cs.ObjectId)
                item['centre_mm'] = [float(v) for v in _origin_coords_mm(cs)]
            except Exception as exc:
                item['warning_control_error'] = utext(exc)[:300]
        result['controls'].append(item)
    result['controls_complete'] = True
    try:
        cs_root = getattr(Model, 'CoordinateSystems', None)
        if cs_root is None or getattr(cs_root, 'Children', None) is None:
            raise RuntimeError('CoordinateSystems.Children unavailable')
        for cs in cs_root.Children:
            name = utext(getattr(cs, 'Name', ''))
            if name.startswith('AutoMesh_QR_CS_'):
                result['warning_coordinate_systems'].append({'name': name, 'object_id': int(cs.ObjectId)})
        result['warning_coordinate_systems_complete'] = True
    except Exception:
        result['warning_coordinate_systems_complete'] = False
    return result


def snapshot():
    result = {'project': utext(ExtAPI.DataModel.Project.ProjectDirectory),
              'bodies': [], 'faces': [], 'named': [], 'controls': [], 'mesh': {},
              'geometry_unit': 'unknown', 'display_unit_system': 'unknown', 'display_length_unit': 'unknown',
              'controls_complete': False, 'warning_coordinate_systems': [],
              'warning_coordinate_systems_complete': False}
    try:
        active_unit = utext(ExtAPI.Application.ActiveUnitSystem)
        result['display_unit_system'] = active_unit
        low_unit = active_unit.lower()
        if 'nmm' in low_unit:
            result['display_length_unit'] = 'mm'
        elif 'umks' in low_unit:
            result['display_length_unit'] = 'um'
        elif 'mks' in low_unit:
            result['display_length_unit'] = 'm'
        elif 'cgs' in low_unit:
            result['display_length_unit'] = 'cm'
        elif 'bin' in low_unit:
            result['display_length_unit'] = 'in'
        elif 'bft' in low_unit:
            result['display_length_unit'] = 'ft'
    except Exception:
        pass
    try:
        result['geometry_unit'] = utext(ExtAPI.DataModel.GeoData.Unit)
    except Exception:
        try:
            result['geometry_unit'] = utext(Model.Geometry.LengthUnit)
        except Exception:
            pass
    stamp = hashlib.sha1()
    records = []
    total_faces=0
    try:
        total_faces=sum(len(body.Faces) for assy in ExtAPI.DataModel.GeoData.Assemblies
                        for part in assy.Parts for body in part.Bodies)
    except Exception:
        total_faces=0
    traversed=0
    report_stride=max(1,total_faces//15)
    emit_progress(12,'读取 Meshing 几何','正在读取面信息、面积和拓扑；0/%s 面' % (total_faces or '?'))
    for assembly in ExtAPI.DataModel.GeoData.Assemblies:
        for part in assembly.Parts:
            for body in part.Bodies:
                body_id = int(body.Id)
                current = {'id': body_id, 'name': utext(body.Name),
                           'volume': float(body.Volume), 'face_count': len(body.Faces)}
                result['bodies'].append(current)
                records.append('%d|%.12g|%d;' % (body_id, current['volume'], current['face_count']))
                for face in body.Faces:
                    entry = {'id': int(face.Id), 'body': body_id,
                             'area': float(face.Area), 'type': utext(face.SurfaceType)}
                    try:
                        entry['centroid'] = [float(x) for x in face.Centroid]
                        entry['bbox'] = [float(x) for x in face.GetBoundingBox()]
                        params = face.ParamAtPoint(face.Centroid)
                        entry['normal'] = [float(x) for x in face.NormalAtParam(params[0], params[1])]
                    except Exception:
                        pass
                    if 'cylind' in entry['type'].lower():
                        radii = []
                        try:
                            for edge in face.Edges:
                                if 'circle' not in utext(edge.CurveType).lower():
                                    continue
                                vertices = list(edge.Vertices)
                                if len(vertices) <= 1 and float(edge.Length) > 0:
                                    radii.append(float(edge.Length)/(2.0*math.pi))
                        except Exception:
                            pass
                        if radii:
                            entry['full_circle_radii'] = radii[:12]
                    result['faces'].append(entry)
                    records.append('%d|%.12g|%s|%s|%s;' % (entry['id'], entry['area'], entry['type'],
                                   repr(entry.get('centroid')), repr(entry.get('bbox'))))
                    traversed+=1
                    if traversed % 32 == 0:
                        cancellation_checkpoint()
                    if total_faces and (traversed % report_stride == 0 or traversed == total_faces):
                        emit_progress(12+int(48.0*traversed/total_faces),'读取 Meshing 几何',
                                      '已读取 %d/%d 面；正在获取面积、面型和空间属性' % (traversed,total_faces))
    if not result['bodies'] or not result['faces']:
        raise RuntimeError('No valid geometry loaded in current Meshing session')
    records.append('unit|%s' % result['geometry_unit'])
    for record in sorted(records):
        stamp.update(record.encode('utf-8'))
    result['fingerprint'] = stamp.hexdigest()
    result['geometry_unit_factor_mm'] = unit_to_mm(result['geometry_unit'])
    nsroot = Model.NamedSelections
    if nsroot:
        for n in nsroot.Children:
            item = {'name': utext(n.Name), 'ids': []}
            try:
                item['ids'] = [int(i) for i in n.Location.Ids]
            except Exception:
                item['warning'] = 'No readable geometry scoping'
            result['named'].append(item)
    result.update(live_control_inventory())
    for field in ('Elements','Nodes','PhysicsPreference','SolverPreference','ElementOrder','ElementSize','MaximumSize','GrowthRate','UseAdaptiveSizing','CaptureCurvature','CaptureProximity','DefeatureSize','UseAdvancedSizeFunction'):
        try:
            result['mesh'][field] = utext(getattr(Model.Mesh, field))
        except Exception:
            pass
    mesh = Model.Mesh
    emit_progress(63,'读取现有控制','几何扫描结束；正在读取命名和已配置 Sizing')
    result['capabilities'] = {
        'read_only_probe': True,
        'generate_mesh': hasattr(mesh,'GenerateMesh'),
        'add_sizing': hasattr(mesh,'AddSizing'),
        'global_capture_proximity': hasattr(mesh,'CaptureProximity'),
        'global_capture_curvature': hasattr(mesh,'CaptureCurvature'),
        'quality_skewness': hasattr(MeshMetricType,'Skewness'),
        'quality_orthogonal_quality': hasattr(MeshMetricType,'OrthogonalQuality'),
        'quality_worksheet_failed_count': hasattr(mesh,'GetVolumeMeshQualityCountFailed'),
        'official_failed_element_group': hasattr(mesh,'CreateMQVolumeElementsNamedSelection'),
        'fluent_msh_export': hasattr(getattr(mesh,'InternalObject',None),'WriteFluentInputFile'),
        'local_sizing_native_properties':'unknown_until_sizing_created',
    }
    return result


def quick_geometry_identity():
    """Same geometry fingerprint as snapshot(), without face normals/edge traversal.

    This still verifies the active project and the actual geometry, but avoids
    doing costly API calls (ParamAtPoint, NormalAtParam, edge loops) twice.
    """
    project=utext(ExtAPI.DataModel.Project.ProjectDirectory)
    unit='unknown'
    try:
        unit=utext(ExtAPI.DataModel.GeoData.Unit)
    except Exception:
        try: unit=utext(Model.Geometry.LengthUnit)
        except Exception: pass
    records=[]
    for assembly in ExtAPI.DataModel.GeoData.Assemblies:
        for part in assembly.Parts:
            for body in part.Bodies:
                bid=int(body.Id)
                records.append('%d|%.12g|%d;' % (bid,float(body.Volume),len(body.Faces)))
                for face in body.Faces:
                    entry={'id':int(face.Id), 'area':float(face.Area),
                           'type':utext(face.SurfaceType)}
                    try:
                        entry['centroid']=[float(x) for x in face.Centroid]
                        entry['bbox']=[float(x) for x in face.GetBoundingBox()]
                    except Exception:
                        pass
                    records.append('%d|%.12g|%s|%s|%s;' %
                        (entry['id'],entry['area'],entry['type'],
                         repr(entry.get('centroid')),repr(entry.get('bbox'))))
    records.append('unit|%s' % unit)
    stamp=hashlib.sha1()
    for r in sorted(records): stamp.update(r.encode('utf-8'))
    return {'project':project,'fingerprint':stamp.hexdigest(),'unit':unit}


def read_only_verified_snapshot(job):
    """Check current project + full geometry fingerprint without rereading all
    faces/normals/edges and WITHOUT rejecting legitimate user Sizing changes.
    Suitable ONLY for readonly quality checks and worksheet marking; destructive
    operations must still reverify their own native controls and scoping.
    """
    prior=read(SNAPSHOT_FILE)
    current=quick_geometry_identity()
    ensure_current(job,current)
    if prior.get('fingerprint')!=current['fingerprint'] or prior.get('project')!=current['project']:
        raise RuntimeError('Current geometry differs from saved scan; rescan before quality check')
    # Mesh/Sizing may change after the original full scan (for example, after
    # the user manually regenerates a mesh). Refresh only live read-only data.
    prior['mesh'] = prior.get('mesh') or {}
    for field in ('Elements','Nodes','ElementSize','MaximumSize','GrowthRate'):
        try: prior['mesh'][field] = utext(getattr(Model.Mesh, field))
        except Exception: pass
    prior.update(live_control_inventory())
    write(SNAPSHOT_FILE, prior)
    return prior


def verify_previous_snapshot(job):
    prior=read(SNAPSHOT_FILE)
    quick=quick_geometry_identity()
    ensure_current(job, quick)
    if prior.get('fingerprint') != quick['fingerprint']:
        raise RuntimeError('Snapshot and current geometry differ, refusing to use cached face data')
    oldnames=[(x.get('name'),tuple(x.get('ids',[]))) for x in prior.get('named',[])]
    current=[]
    root=Model.NamedSelections
    if root:
        for n in root.Children:
            try: current.append((utext(n.Name),tuple(int(i) for i in n.Location.Ids)))
            except Exception: current.append((utext(n.Name),tuple()))
    if oldnames!=current:
        raise RuntimeError('Named selections changed after inspect; scan again')
    oldctrl=[(x.get('name'),tuple(x.get('ids',[])),x.get('mm','')) for x in prior.get('controls',[])]
    now=[]
    for c in Model.Mesh.Children:
        try: ids=tuple(int(i) for i in c.Location.Ids)
        except Exception: ids=tuple()
        try: mm=utext(c.ElementSize)
        except Exception: mm=''
        now.append((utext(c.Name),ids,mm))
    if oldctrl!=now:
        raise RuntimeError('Meshing controls changed after inspect; scan again')
    return prior


def ensure_current(job, state):
    if utext(job.get('expected_project','')).replace('\\','/').lower().rstrip('/') != state['project'].replace('\\','/').lower().rstrip('/'):
        raise RuntimeError('Project mismatch; abort without changes')
    if job.get('geometry_fingerprint') != state['fingerprint']:
        raise RuntimeError('Geometry fingerprint changed; INSPECT again, old face IDs forbidden')


def location(ids):
    scope = ExtAPI.SelectionManager.CreateSelectionInfo(SelectionTypeEnum.GeometryEntities)
    scope.Ids = list(ids)
    return scope


def apply_plan(plan, state):
    m = Model.Mesh
    if int(m.Elements) != 0:
        raise RuntimeError('Mesh already exists: refuse editing existing generated mesh in v0.1')
    if plan['geometry_fingerprint'] != state['fingerprint'] or plan['expected_project'] != state['project']:
        raise RuntimeError('Plan is stale')
    all_ids = set(int(f['id']) for f in state['faces'])
    names = {}
    for control in m.Children:
        names.setdefault(utext(control.Name), []).append(control)
    details = []
    for spec in plan['controls']:
        ids = [int(i) for i in spec['ids']]
        name = utext(spec['control_name'])
        if not name.startswith('AutoMesh_'):
            raise RuntimeError('Refuse modification of non-AutoMesh controls')
        if not ids or not set(ids).issubset(all_ids):
            raise RuntimeError('Invalid face IDs for control %s' % name)
        if len(names.get(name, [])) > 1:
            raise RuntimeError('Duplicate control name: %s' % name)
    for spec in plan['controls']:
        ids = [int(i) for i in spec['ids']]
        name = utext(spec['control_name'])
        if names.get(name):
            c = names[name][0]
            if set(int(i) for i in c.Location.Ids) != set(ids):
                raise RuntimeError('Existing AutoMesh control has a different scope: %s' % name)
    for spec in plan['controls']:
        ids = [int(i) for i in spec['ids']]
        name = utext(spec['control_name'])
        if names.get(name):
            c = names[name][0]
        else:
            c = m.AddSizing()
            c.Name = name
            c.Location = location(ids)
        c.ElementSize = Quantity('%g [mm]' % float(spec['mm']))
        details.append({'name': name, 'size': utext(c.ElementSize), 'count': len(ids)})
    if plan.get('apply_global', False):
        m.ElementSize = Quantity('%g [mm]' % float(plan['global_mm']))
        m.MaximumSize = Quantity('%g [mm]' % float(plan['max_mm']))
        m.GrowthRate = 1.3
    return {'configured': details, 'global_modified': bool(plan.get('apply_global', False)),
            'mesh_elements': int(m.Elements)}


def quality_worksheet_summary():
    """Best effort read-only API (2024 R2). Not a substitute for element IDs."""
    m=Model.Mesh
    info={'status':'unavailable','metrics':[],'source':'2024R2 Mesh Quality Worksheet'}
    if not hasattr(m,'GetVolumeMeshQualityCountFailed'):
        return info
    for metricname in ('Skewness','OrthogonalQuality'):
        metric=getattr(MeshMetricType,metricname,None)
        if metric is None:
            continue
        try:
            failed=int(m.GetVolumeMeshQualityCountFailed(metric))
            warn=int(m.GetVolumeMeshQualityWarningCountFailed(metric))
            info['metrics'].append({'name':metricname,'failed_count':failed,'warning_count':warn})
        except Exception as e:
            info['metrics'].append({'name':metricname,'error':utext(e)[:150]})
    if any('failed_count' in x for x in info['metrics']):
        info['status']='available_read_only'
    return info


def mark_official_quality_elements(kind='failed'):
    """ACT Worksheet element group. Zero count is a valid read-only outcome.

    kind='failed' -> failCode=1, kind='warning' -> failCode=0.
    Never reinterpret sampled geometry as official quality failure.
    """
    if kind not in ('failed','warning'):
        raise ValueError('Unsupported worksheet group kind')
    m=Model.Mesh
    if int(m.Elements)<=0:
        return {'status':'no_mesh','kind':kind,'created_groups':[],
                'message':'当前没有可分析的网格；未创建命名选择'}
    metric=getattr(MeshMetricType,'Skewness',None)
    count_method=('GetVolumeMeshQualityCountFailed' if kind=='failed'
                  else 'GetVolumeMeshQualityWarningCountFailed')
    if metric is None or not hasattr(m,count_method):
        return {'status':'worksheet_unavailable','kind':kind,'created_groups':[],
                'message':'当前 ACT 接口无法读取此类质量单元数量；请在 Meshing Quality Worksheet 中计算并查看'}
    try:
        n=int(getattr(m,count_method)(metric))
    except Exception as exc:
        return {'status':'worksheet_unavailable','kind':kind,'created_groups':[],
                'message':'Quality Worksheet 计数读取失败：'+utext(exc)[:180]}
    if n<=0:
        return {'status':'zero_elements','kind':kind,'metric':'Skewness','worksheet_count':0,
                'created_groups':[],
                'message':('当前 Worksheet 阈值下失败单元数为 0' if kind=='failed' else
                           '当前 Worksheet 阈值下警告单元数为 0')+
                           '；未创建选择组。请确认 Worksheet 已刷新、阈值适用于当前网格。'}
    if not hasattr(m,'CreateMQVolumeElementsNamedSelection'):
        return {'status':'creation_api_unavailable','kind':kind,'worksheet_count':n,
                'created_groups':[], 'message':'官方元素数可读，但当前 ACT 不支持创建质量元素命名组'}
    if n>20000:
        return {'status':'too_many_elements','kind':kind,'worksheet_count':n,
                'created_groups':[], 'message':'质量单元数超过 20000；避免自动创建巨大选择组，请在 Meshing 原生 Worksheet 中定位'}
    def selection_key(x):
        try: return (utext(x.Name),int(x.ObjectId))
        except Exception: return (utext(x.Name),id(x))
    before=set(selection_key(x) for x in Model.NamedSelections.Children)
    m.CreateMQVolumeElementsNamedSelection(metric,1 if kind=='failed' else 0)
    after=set(selection_key(x) for x in Model.NamedSelections.Children)
    created=sorted(after-before)
    sample_ids=[];sample_centroids=[]
    data=None
    try:
        data=ExtAPI.DataModel.MeshDataByName('Global')
    except Exception:
        pass
    for sel in Model.NamedSelections.Children:
        try:key=selection_key(sel)
        except Exception:continue
        if key not in created:continue
        try:element_ids=[int(i) for i in sel.Location.Ids]
        except Exception:element_ids=[]
        for eid in element_ids[:max(0,32-len(sample_ids))]:
            sample_ids.append(eid)
            if data is None:continue
            try:
                elem=data.ElementById(eid)
                nodes=list(elem.Nodes)
                if not nodes:continue
                coords=[]
                for node in nodes:
                    nd=node if hasattr(node,'X') else data.NodeById(int(node))
                    coords.append((float(nd.X),float(nd.Y),float(nd.Z)))
                centre=[round(sum(pt[k] for pt in coords)/len(coords),6) for k in range(3)]
                sample_centroids.append({'element_id':eid,'centroid_mesh_units':centre})
            except Exception:pass
    return {'status':'created' if created else 'api_completed_unenumerated',
            'kind':kind,'metric':'Skewness','worksheet_count':n,
            'worksheet_failed_count':n if kind=='failed' else 0,
            'created_groups':[x[0] for x in created],
            'selected_element_ids_sample':sample_ids,
            'centroids_sample':sample_centroids,
            'message':'ANSYS 质量元素组已创建；请在 Meshing 树中高亮确认' if created else
                      '调用了 ACT 创建接口但未识别新增组；请在 Meshing 树中核查'}



def _selection_enum(enum, candidates, purpose):
    """Runtime ACT enum compatibility; do not guess enumeration integers."""
    for candidate in candidates:
        value=getattr(enum,candidate,None)
        if value is not None:
            return value
    raise RuntimeError(u'当前 ACT 缺少 %s 枚举：%s；未改变网格或质量阈值' %
                       (purpose, '/'.join(candidates)))


def validate_skew_range(low,high):
    low=float(low); high=float(high)
    if (math.isnan(low) or math.isinf(low) or math.isnan(high) or math.isinf(high)
            or low<0 or high>1 or low>=high):
        raise ValueError(u'偏度区间须满足 0 ≤ 下限 < 上限 ≤ 1')
    return low,high


def mark_threshold_skew_elements(low=0.90,high=1.00):
    """Mark actual Mesh Element Skewness between user-selected bounds.

    Uses an ACT Named Selection Worksheet with Skewness criteria, NOT the
    Warning / Error quality classification. Does not touch worksheet warning
    levels, mesh generation, Sizing, or original Named Selections.
    Only removes the newly created selection if generation/validation fails.
    """
    low,high=validate_skew_range(low,high)
    mesh=Model.Mesh
    if int(mesh.Elements)<=0:
        return {'status':'no_mesh','kind':'skew_range','created_groups':[],
                'range_min':low,'range_max':high,
                'message':u'当前没有已生成网格，未创建选择组。'}
    # Use runtime ACT enums, with explicit import for consoles that do not
    # inject them into globals. Never invent numeric enum IDs.
    try:
        from Ansys.Mechanical.DataModel.Enums import (SelectionType,
            SelectionCriterionType, SelectionOperatorType, SelectionActionType,
            GeometryDefineByType)
    except ImportError:
        # ANSYS scripting consoles usually expose these names already.
        SelectionType=globals().get('SelectionType',None)
        SelectionCriterionType=globals().get('SelectionCriterionType',None)
        SelectionOperatorType=globals().get('SelectionOperatorType',None)
        SelectionActionType=globals().get('SelectionActionType',None)
        GeometryDefineByType=globals().get('GeometryDefineByType',None)
    # Never mutate ANY native Quality Worksheet warning/error thresholds.
    etype=_selection_enum(SelectionType,('MeshElement','MeshElements'),u'Mesh Element 类型')
    metric=_selection_enum(SelectionCriterionType,('Skewness',),u'Skewness 筛选')
    # Prefer the native, single-row inclusive interval when ACT exposes it.
    # This is safer than two rows because filter intersection semantics differ
    # between older Meshing Worksheet builds.
    range_op=getattr(SelectionOperatorType,'RangeInclude',None)
    greater=None; less=None; filter_action=None
    if range_op is None:
        greater=_selection_enum(SelectionOperatorType,
            ('GreaterThanOrEqual','GreaterThanOrEqualTo','GreaterThanEqual','GreaterThan'),u'偏度下限比较')
        if high<1.0:
            less=_selection_enum(SelectionOperatorType,
                ('LessThanOrEqual','LessThanOrEqualTo','LessThanEqual','LessThan'),u'偏度上限比较')
            filter_action=_selection_enum(SelectionActionType,('Filter','Intersect'),u'两端筛选交集')
    sheet=_selection_enum(GeometryDefineByType,('Worksheet',),u'Named Selection Worksheet')
    cancellation_checkpoint()
    # Give each completed mark a unique name: never overwrite a manual group.
    suffix=utext(job.get('job_id',''))[:8]
    name='AutoMesh_Skew_%03d_%03d_%s' % (int(round(low*1000)),int(round(high*1000)),suffix)
    target=Model.AddNamedSelection()
    try:
        target.Name=name
        target.ScopingMethod=sheet
        criteria=target.GenerationCriteria
        criteria.Add(None)
        row=criteria[0]
        row.EntityType=etype
        row.Criterion=metric
        if range_op is not None:
            row.Operator=range_op
            # Skewness is unitless. 2024 R2 ACT expects System.Double (IConvertible)
            # rather than Ansys.Core.Units.Quantity for these Worksheet fields.
            row.LowerBound=float(low)
            row.UpperBound=float(high)
        else:
            row.Operator=greater
            # Same unitless numeric contract for the two-row ACT fallback.
            row.Value=float(low)
            if high<1.0:
                criteria.Add(None)
                upper_row=criteria[1]
                upper_row.Action=filter_action
                upper_row.EntityType=etype
                upper_row.Criterion=metric
                upper_row.Operator=less
                upper_row.Value=float(high)
        cancellation_checkpoint()
        target.Generate()
        ids=sorted(set(int(i) for i in target.Location.Ids))
        if len(ids)>50000:
            target.Delete()
            return {'status':'too_many_elements','kind':'skew_range','created_groups':[],
                    'range_min':low,'range_max':high,'worksheet_count':len(ids),
                    'message':u'精确筛选到 %d 个单元，超过 32GB 设备的 50000 单元自动标记保护上限，已撤销本次新选择组；请缩窄偏度区间。' % len(ids)}
        if not ids:
            target.Delete()
            return {'status':'zero_elements','kind':'skew_range','created_groups':[],
                    'range_min':low,'range_max':high,'worksheet_count':0,
                    'message':u'当前网格此偏度区间未选中单元。若最大偏度位于区间内，请在 Meshing 检查 Mesh Metric/Worksheet 数据是否已刷新。'}
        # Ensure the returned object is still a genuine Mesh Element selection.
        loc_type=getattr(target.Location,'SelectionType',None)
        selection_enum=globals().get('SelectionTypeEnum',None)
        selection_mesh=getattr(selection_enum,'MeshElements',None)
        if loc_type is not None and selection_mesh is not None and loc_type != selection_mesh:
            raise RuntimeError(u'Named Selection 返回的实体不是 Mesh Elements；拒绝错误标记')
        return {'status':'created','kind':'skew_range','metric':'Skewness',
                'range_min':low,'range_max':high, 'worksheet_count':len(ids),
                'selection_source':'named_selection_worksheet_skewness_range',
                'created_groups':[utext(target.Name)],
                'selected_element_ids_sample':ids[:32],
                'message':u'已按 Mesh Element → Skewness 区间 %.3g～%.3g 创建 %d 单元的独立选择组。后续方案将读取这批单元，不依赖默认警告数。' % (low,high,len(ids))}
    except Exception:
        # Limit cleanup to the single NEW group created by this transaction.
        try: target.Delete()
        except Exception: pass
        raise


def mark_official_failed_elements():
    """Backward-compatible quality action."""
    return mark_official_quality_elements('failed')


def probe_tet_geometry(max_samples=6000):
    """Sample mesh element geometry and return real IDs/centroids of *sampled* tets.

    Mean-ratio shape proxy is not official Skewness or Fluent OrthogonalQuality.
    This diagnostic NEVER claims exhaustive localization or changes a mesh.
    """
    output={'status':'unavailable','metric':'tet_mean_ratio_shape_proxy',
            'official_quality_metric':False, 'max_samples':max_samples,'samples':[]}
    try:
        data=ExtAPI.DataModel.MeshDataByName('Global')
        num=int(data.ElementCount)
        output['total_elements']=num
        if num<1:return output
        ids=data.ElementIds
        step=max(1,int(math.ceil(float(num)/max_samples)))
        processed=0; tets=0; risk=[]
        for j in range(0,num,step):
            if processed >= max_samples:break
            processed+=1
            if processed % max(1,max_samples//10)==0:
                emit_progress(70+int(22.0*processed/max_samples),'采样风险单元',
                              '已检查 %d/%d 个采样目标；实际网格总数 %d' % (processed,max_samples,num))
            el=data.ElementById(int(ids[j]))
            if el is None:continue
            kind=utext(el.Type).lower()
            if 'tet' not in kind and 'tetra' not in kind:continue
            nodes=list(el.Nodes)
            if len(nodes)<4:continue
            coords=[]
            for node in nodes[:4]:
                n=node if hasattr(node,'X') else data.NodeById(int(node))
                coords.append((float(n.X),float(n.Y),float(n.Z)))
            e=[]
            for aa in range(4):
                for bb in range(aa+1,4):
                    e.append(sum((coords[aa][k]-coords[bb][k])**2 for k in range(3)))
            a,b,c,d=coords
            ab=[b[k]-a[k] for k in range(3)]
            ac=[c[k]-a[k] for k in range(3)]
            ad=[d[k]-a[k] for k in range(3)]
            cross=(ac[1]*ad[2]-ac[2]*ad[1],ac[2]*ad[0]-ac[0]*ad[2],ac[0]*ad[1]-ac[1]*ad[0])
            volume=abs(sum(ab[k]*cross[k] for k in range(3)))/6.0
            sume=sum(e)
            q=12.0*(3.0*volume)**(2.0/3.0)/sume if sume>0 else 0.0
            tets+=1
            if q<0.20:
                centroid=[round(sum(x[k] for x in coords)/4.0,6) for k in range(3)]
                risk.append({'element_id':int(ids[j]),'centroid_mesh_units':centroid,
                             'shape_proxy':round(q,5)})
        risk.sort(key=lambda x:x['shape_proxy'])
        output['samples']=risk[:25]
        output['checked_element_count']=processed
        output['checked_tet_count']=tets
        output['sampling_fraction']=round(float(processed)/num,6)
        output['mesh_unit']=utext(getattr(data,'Unit','unknown'))
        output['status']='sampled_not_exhaustive' if processed<num else 'all_elements_inspected'
        output['warning']='Sampled tetrahedral geometry only: NOT official failed cell set, and may miss worst cells'
    except Exception as e:
        output['error']=utext(e)[:250]
    return output


def assess_quality():
    """Read official ACT mesh metrics and report why orthogonal quality is absent.

    Meshing GUI may offer Orthogonal Quality even when this ACT context does not
    expose MeshMetricType.OrthogonalQuality. Neither OrthogonalAngle nor
    1-Skewness is an acceptable replacement for the actual CFD metric.
    """
    m=Model.Mesh
    if int(m.Elements)<=0:
        raise RuntimeError('Mesh not generated')
    q={'elements':int(m.Elements),'nodes':int(m.Nodes),'warnings':[]}
    diagnostic={'status':'not_measured',
                'interface':'MeshMetricType.OrthogonalQuality',
                'reason':'Orthogonal quality measurement not yet attempted',
                'action':'Verify native Orthogonal Quality in Meshing or Fluent'}
    q['orthogonal_quality_diagnostic']=diagnostic
    original=None
    original_readable=False
    try:
        original=m.MeshMetric
        original_readable=True
    except Exception as e:
        # Retain the original worker's best-effort read, but disclose that
        # restoring the prior metric cannot be independently verified.
        q['warnings'].append('Original MeshMetric unreadable; restore cannot be verified: '+utext(e))
    try:
        try:
            skew_enum=getattr(MeshMetricType,'Skewness',None)
            if skew_enum is None:
                q['warnings'].append('ACT MeshMetricType.Skewness unavailable')
            else:
                m.MeshMetric=skew_enum
                q['skewness']={'min':float(m.Minimum.Value),
                               'max':float(m.Maximum.Value),
                               'mean':float(m.Average.Value),
                               'source':'ACT MeshMetricType.Skewness'}
        except Exception as e:
            q['warnings'].append('Skewness unavailable: '+utext(e))
        orthogonal_enum=getattr(MeshMetricType,'OrthogonalQuality',None)
        if orthogonal_enum is None:
            diagnostic['status']='enum_not_exposed'
            diagnostic['reason']='The active ACT MeshMetricType enum does not expose OrthogonalQuality'
            diagnostic['action']='Check native Meshing Orthogonal Quality or a Fluent mesh-quality report; never infer it from skewness'
            q['warnings'].append('ACT OrthogonalQuality enum missing; no OQ result returned (not a value of 0)')
        else:
            try:
                m.MeshMetric=orthogonal_enum
                values=[float(m.Minimum.Value),float(m.Maximum.Value),float(m.Average.Value)]
                if any(math.isnan(v) or math.isinf(v) or v<0 or v>1 for v in values) or values[0]>values[1]:
                    raise ValueError('OQ values outside 0..1 or reversed: '+utext(values))
                q['orthogonal_quality']={'min':values[0], 'max':values[1],
                                         'mean':values[2],
                                         'source':'ACT MeshMetricType.OrthogonalQuality'}
                diagnostic['status']='measured'
                diagnostic['reason']='Orthogonal Quality was measured using native ACT MeshMetricType.OrthogonalQuality'
                diagnostic['action']='Check Fluent when export changes internal face-zone topology'
            except Exception as e:
                diagnostic['status']='metric_read_failed'
                diagnostic['reason']=utext(e)[:350]
                diagnostic['action']='Review the ACT exception; no substitute values were calculated'
                q['warnings'].append('OrthogonalQuality ACT read failed: '+utext(e))
    finally:
        if original_readable:
            try:
                m.MeshMetric=original
            except Exception as e:
                q['warnings'].append('MeshMetric restoration failed: '+utext(e))
    if 'skewness' in q and q['skewness']['max']>0.95:
        q['warnings'].append('max skewness > 0.95: locate critical cells')
    if 'orthogonal_quality' in q and q['orthogonal_quality']['min']<0.1:
        q['warnings'].append('min orthogonal quality < 0.1: locate risky cells')
    q['current_mesh_only']=True
    return q



def geometry_sizing_candidates(state, base_mm):
    return classify(state, base_mm)


def editable_group_candidates(state, base_mm):
    return classify_face_areas(state, base_mm)


def try_native_size_function(control, group):
    """Reuse documented 2024 Meshing local sizing functions where available.

    A failed optional property never pretends that proximity or curvature is on.
    The face ElementSize remains as a conservative fallback.
    """
    mode=group.get('native_function','uniform')
    result={'requested':mode,'applied':'uniform_fallback'}
    if mode not in ('proximity','curvature'):
        result['applied']='uniform'
        return result
    field='CaptureProximity' if mode=='proximity' else 'CaptureCurvature'
    original_value = None
    old_gap = None
    try:
        original_value=getattr(control,field)
        if mode=='proximity':old_gap=getattr(control,'ProximityGapFactor')
    except Exception: pass
    try:
        from System import Boolean, Enum
        attr=control.GetType().GetProperty(field)
        if attr is None:
            result['reason']='ACT property not found'
            return result
        kind=attr.PropertyType
        if kind == Boolean:
            setattr(control,field,True)
        elif kind.IsEnum:
            enumval=None
            for name in ('Yes','True','On'):
                try:
                    enumval=Enum.Parse(kind,name,True)
                    break
                except Exception:
                    continue
            if enumval is None:
                result['reason']='No supported yes enum'
                return result
            setattr(control,field,enumval)
        else:
            result['reason']='Unknown ACT property type'
            return result
        if mode=='proximity':
            factor=float(group.get('cells_per_gap',2.5))
            control.ProximityGapFactor=factor
            result['cells_per_gap']=factor
        result['applied']=mode
    except Exception as error:
        result['reason']=utext(error)[:150]
        try:
            if original_value is not None:setattr(control,field,original_value)
            if old_gap is not None:setattr(control,'ProximityGapFactor',old_gap)
        except Exception as revert_error:
            result['applied']='unverified_partial_native_sizing'
            result['rollback_error']=utext(revert_error)[:150]
    return result


def named_boundary_check(state):
    """Require two trusted, disjoint face-scoped BC selections before altering mesh."""
    import re
    valid = set(int(face['id']) for face in state.get('faces', []))
    names = {'inlet': set(), 'outlet': set()}
    for ns in state.get('named', []):
        name = utext(ns.get('name','')).strip().lower()
        match = re.match(r'^(inlet|outlet)(?:[ _-]?[0-9]+)?$', name)
        if match is None:
            continue
        role=match.group(1)
        ids = [int(i) for i in ns.get('ids', [])]
        if not ids or len(ids)>5000 or not set(ids).issubset(valid):
            raise RuntimeError('Named Selection %s must contain valid geometry faces; please rescan' % name)
        names[role].update(ids)
    missing = [role for role in ('inlet','outlet') if not names[role]]
    if missing:
        raise RuntimeError('Please name face selections inlet and outlet in Meshing before generating: %s missing. Scan again after naming.' % ', '.join(missing))
    if names['inlet'].intersection(names['outlet']):
        raise RuntimeError('inlet and outlet Named Selections overlap. Please correct their face scopes.')
    return {'inlet_faces':len(names['inlet']), 'outlet_faces':len(names['outlet']),
            'validated':'named face scoping only; no automatic BC physical settings'}


def verify_live_boundaries(state):
    """Fail closed on current face scoping AND SendToSolver before mutation/export.

    Cached scan results are not an authorization to drop Fluent boundaries.
    """
    named=[]
    seen_names=set()
    for obj in Model.NamedSelections.Children:
        name=utext(obj.Name)
        if not __import__('re').match(r'^(inlet|outlet)(?:[ _-]?[0-9]+)?$', name.strip().lower()):
            continue
        lowered=name.strip().lower()
        if lowered in seen_names:
            raise RuntimeError('Duplicate named boundary %s; use unique inlet/outlet names' % name)
        seen_names.add(lowered)
        try:
            ids=[int(i) for i in obj.Location.Ids]
        except Exception:
            raise RuntimeError('Named boundary %s has unreadable face scope' % name)
        try:
            transfer=obj.SendToSolver
        except Exception:
            raise RuntimeError('Cannot verify Send to Solver for %s; refusing operation' % name)
        if transfer is not True and utext(transfer).lower() not in ('true','1'):
            raise RuntimeError('Named boundary %s has Send to Solver disabled; enable it before generating or exporting' % name)
        named.append({'name':name,'ids':ids})
    verified=dict(state)
    verified['named']=named
    outcome=named_boundary_check(verified)
    outcome['send_to_solver_verified']=True
    return outcome


def reviewed_quality_gate(quality):
    """Three-state preliminary ACT quality check, not a Fluent certification."""
    def metric(section,key):
        item=quality.get(section)
        if not isinstance(item,dict) or key not in item:
            return None
        try:
            number=float(item[key])
            if math.isnan(number) or math.isinf(number):return None
            return number
        except (ValueError,TypeError,OverflowError):return None
    skew=metric('skewness','max')
    oq=metric('orthogonal_quality','min')
    warnings=[]
    if skew is None:warnings.append('Skewness unavailable')
    if oq is None:warnings.append('CFD orthogonal quality unavailable; verify in Fluent')
    if skew is not None and skew>0.95:
        return {'status':'fail','quality_gate':'critical_skewness',
                'reason':'Skewness > 0.95; locate failed elements before export'}
    if oq is not None and oq<0.1:
        return {'status':'fail','quality_gate':'critical_orthogonal_quality',
                'reason':'Orthogonal Quality < 0.1; inspect mesh before export'}
    if skew is None or oq is None or skew>0.90 or oq<0.20:
        if skew is not None and skew>0.90:warnings.append('Skewness > 0.90')
        if oq is not None and oq<0.20:warnings.append('Orthogonal Quality < 0.20')
        return {'status':'warning','quality_gate':'review_required',
                'reason':'; '.join(warnings) or 'Boundary/mesh quality requires human review'}
    return {'status':'pass','quality_gate':'preliminary_pass',
            'reason':'ACT preliminary checks passed; Fluent validation still required'}


def guarded_export(job,state):
    """Explicit separate export; never overwrites, never certifies Fluent validity."""
    verified=verify_live_boundaries(state)
    if int(Model.Mesh.Elements)<=0:
        raise RuntimeError('Export requires a generated, non-empty mesh')
    path=utext(job.get('export_path','')).strip()
    if not path or not path.lower().endswith('.msh') or not os.path.isabs(path) or any(ord(c)>127 for c in path):
        raise RuntimeError('Export requires an explicit ASCII path ending .msh')
    path=os.path.abspath(path)
    if not os.path.isdir(os.path.dirname(path)):
        raise RuntimeError('Export directory does not exist; never create folders automatically')
    if os.path.exists(path):
        raise RuntimeError('Export target already exists; choose a new filename (no overwrite)')
    quality=assess_quality()
    verdict=reviewed_quality_gate(quality)
    if verdict['status']=='fail':
        raise RuntimeError('Export rejected by quality gate: '+verdict['reason'])
    if not job.get('quality_review_acknowledged'):
        raise RuntimeError('Explicit quality review acknowledgement required before export')
    exporter=getattr(getattr(Model.Mesh,'InternalObject',None),'WriteFluentInputFile',None)
    if not callable(exporter):
        raise RuntimeError('ANSYS Fluent export API unavailable; no file created')
    exporter(path)
    if not os.path.isfile(path) or os.path.getsize(path)<=0:
        raise RuntimeError('Fluent .msh file missing or empty after export')
    return {'output':path, 'export_bytes':os.path.getsize(path),
            'quality':quality,'quality_status':verdict['status'],
            'quality_gate':verdict['quality_gate'], 'boundary':verified,
            'export_validation':'provisional__Fluent_check_required',
            'next_action':'Load into Fluent, verify mesh and boundary zones before use'}


def local_refinement_rules(job):
    plan = job.get('plan') or {}
    overrides = plan.get('local_overrides') or {}
    disabled = plan.get('disabled_regions') or []
    allowed = set(['narrow_parallel','circular_cylindrical','curved_surface','named_inlet','named_outlet','area_small','area_repeated','area_review','named_existing'])
    if not isinstance(overrides,dict) or not isinstance(disabled,list):
        raise RuntimeError('Invalid local refinement plan')
    for name, value in overrides.items():
        if name not in allowed:
            raise RuntimeError('Unknown local sizing category: %s' % name)
        v = float(value)
        if math.isnan(v) or math.isinf(v):
            raise RuntimeError('Non-finite size override')
        if not 0.35 <= v <= 500:
            raise RuntimeError('Local sizing must be between 0.35 and 500 mm')
    if not set(disabled).issubset(allowed):
        raise RuntimeError('Unknown disabled sizing category')
    return overrides, set(disabled)


def _read_control_mm(control):
    """Only trust an explicit and recognized length unit on a prior control."""
    import re
    value = utext(control.ElementSize).strip()
    match = re.search(r'([-+]?\d+(?:\.\d+)?(?:[Ee][-+]?\d+)?)\s*\[?\s*(mm|cm|m|um|in|inch)\b',value,re.I)
    if match is None:
        raise RuntimeError('Existing AutoMesh sizing has an unreadable unit: %s' % value[:70])
    factor = unit_to_mm(match.group(2))
    if factor is None:
        raise RuntimeError('Unrecognized existing sizing unit')
    return float(match.group(1))*factor


def _effective_sizing_ids(control):
    """Read GeometrySelection or Named Selection Location without UI property writes."""
    ids = set()
    for field in ('Location', 'NamedSelection'):
        try:
            scope = getattr(control, field)
            direct = getattr(scope, 'Ids', None)
            if direct is None:
                direct = scope.Location.Ids
            ids.update(int(i) for i in direct)
        except Exception:
            pass
    if not ids:
        raise RuntimeError('Cannot read geometry scope for existing Sizing: '+utext(control.Name))
    return ids
def _is_control_suppressed(control):
    # Mechanical may expose Suppressed as a bool or enum/text; do not assume
    # unavailable means suppressed. Fail safe: such a control is still active.
    try:
        value = control.Suppressed
        if isinstance(value, bool):
            return value
        return utext(value).strip().lower() in ('true', 'yes', 'suppressed', '1')
    except Exception:
        return False

def _is_sizing(control):
    try:
        return 'sizing' in utext(control.DataModelObjectCategory).lower()
    except Exception:
        return False


def _global_meshing_size_mm(job, may_apply_global):
    """Use the actual effective ANSYS global ElementSize, not a UI reference.

    If application will explicitly overwrite global sizing, use that value.
    Otherwise fail closed when effective ANSYS global size cannot be read.
    """
    if may_apply_global:
        size=float(job.get('base_mm',12.0))
    else:
        try:
            size=_read_control_mm(Model.Mesh)
        except Exception as exc:
            raise RuntimeError('Cannot verify current Meshing global ElementSize. Set an explicit length in Meshing or enable approved global override; never assume UI reference size: %s' % utext(exc))
    if math.isnan(size) or math.isinf(size) or not 0.35 <= size <= 500:
        raise RuntimeError('Actual Meshing global ElementSize outside conservative 0.35-500mm budget range: %s' % size)
    return size


def _global_budget_for_local_publication(job):
    """Return a *verified* global size, or None for an automatic/unset value.

    Reading Mesh.ElementSize may yield 0 mm when Mechanical uses automatic
    sizing. Zero is not a physical positive element size, and MUST NOT be
    substituted with the reference slider (`base_mm`). Local-control
    publication does not alter/generate a mesh, so the independent local
    surface budget can still be checked. Generation remains strictly gated.
    """
    if job.get('apply_global', False):
        return (_global_meshing_size_mm(job, True), 'explicit_user_override', '')
    try:
        return (_global_meshing_size_mm(job, False), 'verified_explicit', '')
    except RuntimeError:
        import re
        try:
            raw = utext(Model.Mesh.ElementSize).strip()
        except Exception:
            raise RuntimeError('Cannot read Meshing global ElementSize; local-control budget cannot be characterized safely')
        lower = raw.lower().replace('_', ' ').replace('-', ' ')
        is_auto = bool(re.match(r'^\s*(?:default|automatic|auto|program\s*controlled|programcontrolled)\s*$',lower))
        try:
            # A recognized-unit value of exactly zero is an unresolved
            # automatic/default size, not a real 0-mm meshing instruction.
            is_auto = is_auto or (_read_control_mm(Model.Mesh) == 0.0)
        except Exception:
            pass
        if not is_auto:
            # Invalid *explicit* positive sizes (e.g. 0.05 mm) and any other
            # malformed quantity must retain the original strict rejection.
            return (_global_meshing_size_mm(job, False), 'verified_explicit', '')
        return (None, 'automatic_or_unresolved',
                'Meshing global ElementSize reports %s. AutoMesh has NOT verified its actual generated size; '
                'global volume/memory veto was NOT applied. Only local surface refinement was checked. '
                'Inspect current global sizing in Meshing before reviewed generation.' % raw[:70])


def exact_area_worksheet_scope(spec, state):
    """Preflight the native Face/Size criterion against ALL face areas.

    Worksheet has model-wide scope; a range may include other bodies, user
    selections, or different surface types. Never claim a safe match then.
    """
    if spec.get('category') not in ('area_small','area_repeated','area_review'):
        return None
    facegroup=spec.get('group') or {}
    lo=facegroup.get('area_mm2_min');hi=facegroup.get('area_mm2_max')
    if lo is None or hi is None:return None
    factor=unit_to_mm(state.get('geometry_unit'))
    if not factor:return None
    lo=float(lo);hi=float(hi)
    if lo<=0 or hi<lo:return None
    pad=max(0.0001,hi*0.00001)
    lo=max(0.0,lo-pad);hi=hi+pad
    matching=set()
    for f in state.get('faces',[]):
        try:
            area=float(f['area'])*factor*factor
            if lo<=area<=hi:matching.add(int(f['id']))
        except (KeyError,ValueError,TypeError,OverflowError):pass
    if matching!=set(spec['ids']):return None
    return (lo,hi)


def create_verified_area_selection(spec, state):
    """Use ANSYS 2024R2 native Worksheet Face/Size/RangeInclude if exact.

    If unsupported or a scope cannot match, fall back to explicit geometry
    ID scoping. Always verify final face IDs and SendToSolver, never silently
    accept extra face selections. Existing named selections are untouched.
    """
    name=spec['ns_name'];expected=set(spec['ids']);attempt_note=''
    area_range=exact_area_worksheet_scope(spec,state)
    native_support=(area_range is not None and
                    hasattr(GeometryDefineByType,'Worksheet') and
                    'SelectionType' in globals() and
                    'SelectionCriterionType' in globals() and
                    'SelectionOperatorType' in globals() and
                    'SelectionActionType' in globals())
    if native_support:
        ns=Model.AddNamedSelection();ns.Name=name
        try:
            if utext(ns.Name)!=utext(name):
                raise RuntimeError('ANSYS did not preserve Unicode auxiliary name: '+utext(name))
            ns.ScopingMethod=GeometryDefineByType.Worksheet
            ws=ns.GenerationCriteria
            ws.Add(None)
            row=ws[0]
            row.Active=True
            row.Action=SelectionActionType.Add
            row.EntityType=SelectionType.GeoFace
            row.Criterion=SelectionCriterionType.Size
            row.Operator=SelectionOperatorType.RangeInclude
            row.LowerBound=Quantity('%.12g [mm mm]' % area_range[0])
            row.UpperBound=Quantity('%.12g [mm mm]' % area_range[1])
            ns.Generate()
            if _ids(ns)!=expected:
                raise RuntimeError('native worksheet selected %d instead of %d faces'% (len(_ids(ns)),len(expected)))
            ns.SendToSolver=False
            if bool(ns.SendToSolver):raise RuntimeError('Cannot disable SendToSolver')
            return ns,'native_worksheet_face_area','Face Size RangeInclude [mm^2]'
        except Exception as exc:
            attempt_note=utext(exc)
            try:
                ns.Delete()
            except Exception:
                raise RuntimeError('Native area worksheet failed and the incomplete new selection could not be removed: '+attempt_note)
    ns=Model.AddNamedSelection();ns.Name=name
    if utext(ns.Name)!=utext(name):
        try: ns.Delete()
        except Exception: pass
        raise RuntimeError('ANSYS did not preserve Unicode auxiliary name: '+utext(name))
    ns.Location=location(spec['ids']);ns.SendToSolver=False
    if _ids(ns)!=expected or bool(ns.SendToSolver):
        raise RuntimeError('Geometry selection verification failed: '+name)
    reason=('native worksheet would include other faces or cannot match a body-specific subset' if area_range is None else
            'native worksheet API unavailable or rejected: '+attempt_note)
    return ns,'verified_face_ids',reason


def group_refinement_rules(job, groups):
    """Optional independent overrides with keys returned by read-only INSPECT."""
    plan=job.get('plan') or {}
    values=plan.get('group_overrides') or {}
    disabled=plan.get('disabled_groups') or []
    enabled=plan.get('enabled_groups') or []
    if not isinstance(values,dict) or not isinstance(disabled,list) or not isinstance(enabled,list):
        raise RuntimeError('Invalid per-group refinement plan')
    allowed=set(x.get('group_key') for x in groups)
    unknown=(set(values)|set(disabled)|set(enabled))-allowed
    if unknown:
        raise RuntimeError('Unknown or stale local group keys: '+', '.join(sorted(str(k) for k in unknown)[:6]))
    for key,value in values.items():
        number=float(value)
        if math.isnan(number) or math.isinf(number) or not 0.35<=number<=500:
            raise RuntimeError('Invalid requested per-group size (0.35-500 mm): '+utext(key))
    if set(disabled)&set(enabled):
        raise RuntimeError('One group cannot be both enabled and disabled')
    return values,set(disabled),set(enabled)


def _check_resume_control(control, expected_ids, expected_mm):
    """No edits to pre-existing controls; only exact scope/size may be reused."""
    try:
        ids = set(int(i) for i in control.Location.Ids)
    except Exception:
        raise RuntimeError('Old AutoMesh control has incomplete face scope: %s' % utext(control.Name))
    if ids != set(expected_ids):
        raise RuntimeError('Old AutoMesh control face scope differs: %s; manual inspection required' % utext(control.Name))
    prior_mm = _read_control_mm(control)
    if abs(prior_mm - float(expected_mm)) > max(0.0001, 0.001*expected_mm):
        raise RuntimeError('Old AutoMesh control size differs: %s (%g vs %g mm)' %
                           (utext(control.Name), prior_mm, expected_mm))
    return {'name':utext(control.Name),'mm':prior_mm,'count':len(ids)}


def automatic_mesh(job, state):
    m = Model.Mesh
    base = float(job.get('base_mm', 12.0))
    existing_mesh = int(m.Elements) > 0
    groups, evidence = geometry_sizing_candidates(state, base)
    boundary = verify_live_boundaries(state)
    overrides, disabled = local_refinement_rules(job)
    evidence['explicit_boundaries'] = boundary
    evidence['disabled_regions'] = sorted(disabled)
    evidence['local_overrides_mm'] = overrides
    evidence['physics_preference'] = utext(m.PhysicsPreference)
    evidence['solver_preference'] = utext(m.SolverPreference)
    if 'CFD' not in evidence['physics_preference'].upper() or 'FLUENT' not in evidence['solver_preference'].upper():
        raise RuntimeError('Expected CFD + Fluent mesh settings. Do not silently change solver preference')
    all_controls = list(m.Children)
    prior_owned = {}
    manual = []
    for control in all_controls:
        name = utext(control.Name)
        if name.startswith('AutoMesh_'):
            if name in prior_owned:
                raise RuntimeError('Duplicate unfinished AutoMesh controls: %s' % name)
            prior_owned[name] = control
        else:
            manual.append(control)
    resume = bool((job.get('plan') or {}).get('resume_owned_controls',False))
    if prior_owned and not existing_mesh and not resume:
        raise RuntimeError('Previous unfinished AutoMesh sizing controls exist: %s. First scan and inspect; tick explicit matching-controls recovery only if the previous task has stopped. No automatic deletion.' % ', '.join(sorted(prior_owned)[:6]))
    manual_controls = bool(manual)
    if manual_controls and job.get('apply_global', False):
        evidence['warnings'].append('User-defined mesh controls present; global sizing preserved')
    if (not existing_mesh and not manual_controls and
        evidence.get('repeated_passage_pairs',0)>=80 and
        not any(g.get('category')=='narrow_parallel' for g in groups)):
        raise RuntimeError('Many repeated narrow channels found but no safe local gap sizing available; do not waste an entire mesh run. Review geometry candidates')

    manual_occupied = set()
    manual_budget=[]
    for control in manual:
        if _is_sizing(control):
            ids=_effective_sizing_ids(control)
            manual_occupied.update(ids)
            manual_budget.append({'category':'additional_manual','group_key':'manual:'+utext(control.Name),
                                  'ids':sorted(ids),'mm':_read_control_mm(control)})
    baseline=_global_meshing_size_mm(job, bool(job.get('apply_global',False) and not manual_controls and not existing_mesh))
    resource = evaluate_refinement_budget(state, manual_budget+groups, overrides, disabled,
                                          base_mm=baseline, enforce_global=True)
    evidence['resource_preflight'] = resource

    specs = []
    occupied = set(manual_occupied)
    for ix, gr in enumerate(groups):
        if gr['category'] in disabled:
            continue
        available = sorted(set(int(fid) for fid in gr['ids']) - occupied)
        if not available:
            continue
        name = 'AutoMesh_%s_%d' % (gr['category'], ix+1)
        size = float(overrides.get(gr['category'], gr['mm']))
        specs.append((name,gr,available,size))
        occupied.update(available)
    expected_names = set(spec[0] for spec in specs)
    unexpected = set(prior_owned) - expected_names
    if unexpected and not existing_mesh:
        raise RuntimeError('Unexpected old AutoMesh controls: %s; inspect manually before retry' % ', '.join(sorted(unexpected)[:6]))
    checked = []
    if not existing_mesh:
        for name, gr, ids, size in specs:
            if name in prior_owned:
                checked.append(_check_resume_control(prior_owned[name],ids,size))

    details = []
    if not existing_mesh:
        for name, gr, ids, target_mm in specs:
            if name in prior_owned:
                details.append({'name':name,'mm':target_mm,'face_count':len(ids),
                    'reason':'Reused exact matching existing AutoMesh Sizing; no edit performed',
                    'native_sizing':{'applied':'preexisting_verified'}})
                continue
            c = m.AddSizing()
            c.Name = name
            c.Location = location(ids)
            c.ElementSize = Quantity('%g [mm]' % target_mm)
            native_status = try_native_size_function(c,gr)
            if native_status.get('applied')=='unverified_partial_native_sizing':
                raise RuntimeError('Native sizing rollback failed for %s; refuse GenerateMesh and inspect manually' % name)
            details.append({'name':name,'mm':target_mm,'face_count':len(ids),
                'diameter_mm_min':gr.get('diameter_mm_min'),
                'diameter_mm_max':gr.get('diameter_mm_max'),
                'reason':gr['reason'],'native_sizing':native_status})
    global_changed = False
    if not existing_mesh and job.get('apply_global', False) and not manual_controls:
        m.ElementSize = Quantity('%g [mm]' % base)
        m.MaximumSize = Quantity('%g [mm]' % (2.0*base))
        m.GrowthRate = 1.3
        global_changed = True
    summary = {'configured':details,'geometry_evidence':evidence,
               'global_sizing_changed':global_changed,'base_mm_reference':base,
               'used_existing_mesh':existing_mesh,
               'existing_user_controls_preserved':True,
               'capabilities':state.get('capabilities',{}),
               'added_control_count':len([d for d in details if d.get('native_sizing',{}).get('applied')!='preexisting_verified']),
               'reused_control_count':len(checked),
               'quality_rule':'CFD orthogonal quality must be independently verified',
               'export_guarantee':False}
    write(RESULT_FILE, {'job_id':job['job_id'],'phase':'auto',
                        'status':'in_progress', 'stage':'quality' if existing_mesh else 'meshing',
                        'summary':summary})
    start = time.time()
    if not existing_mesh:
        write(RESULT_FILE, {'job_id':job['job_id'],'phase':'auto',
                            'status':'in_progress','stage':'GenerateMesh','summary':summary})
        m.GenerateMesh()
    if int(m.Elements) <= 0:
        raise RuntimeError('No valid mesh available after GenerateMesh')
    diagnostic_messages=[]
    try:
        for msg in ExtAPI.Application.Messages:
            if len(diagnostic_messages)>=60:
                break
            item={'message':utext(msg.DisplayString)[:320]}
            try:
                item['severity']=utext(msg.Severity)
            except Exception:
                pass
            try:
                item['face_or_entity_ids']=[int(x) for x in msg.Location.Ids][:60]
            except Exception:
                pass
            diagnostic_messages.append(item)
    except Exception:
        pass
    summary['meshing_messages']=diagnostic_messages
    message_face_ids=sorted(set(i for d in diagnostic_messages
        for i in d.get('face_or_entity_ids',[])))
    body_count=len(state.get('bodies',[]))
    summary['repair_scope']={
        'candidate_geometry_entity_ids':message_face_ids[:100],
        'source':'ANSYS meshing messages only; not a complete bad-element map',
        'fluid_body_count':body_count,
        'automatic_local_remesh_performed':False,
        'selection_scope':'whole body or part according to Workbench Meshing supported methods',
        'reason':'Single fluid body requires body-level remesh; Mesh Quality Worksheet is needed for exact failed-cell selection'
            if body_count<=1 else 'Multi-body selective meshing may be possible after validating exact geometry/body scope',
        'warning':'No automatic deletion, regeneration or remesh of an existing mesh'
    }
    quality = assess_quality()
    summary.update({'seconds':round(time.time()-start,1),
                    'quality':quality, 'mesh_elements':int(m.Elements)})
    if quality.get('skewness',{}).get('max',0)>0.95:
        summary['worksheet_diagnostics']=quality_worksheet_summary()
    skew = quality.get('skewness',{}).get('max')
    preliminary = (skew is not None and skew <= 0.95)
    oq = quality.get('orthogonal_quality',{}).get('min')
    if not preliminary:
        summary['quality_gate'] = 'review_required'
        summary['next_action'] = 'Locate skewness hot-spots in Meshing; no blind automatic remesh' 
    elif oq is None:
        summary['quality_gate'] = 'oq_unverified_manual_review'
        summary['next_action'] = 'Use Meshing/Fluent Orthogonal Quality metric before certifying or exporting' 
    elif oq < 0.1:
        summary['quality_gate'] = 'review_required'
        summary['next_action'] = 'Inspect low Orthogonal Quality cells; no blind automatic remesh'
    else:
        summary['quality_gate'] = 'preliminary_pass'
    path = job.get('export_path','')
    if path and job.get('quality_review_acknowledged') and preliminary and summary['quality_gate'] in ('preliminary_pass','oq_unverified_manual_review'):
        path = os.path.abspath(path)
        if not path.lower().endswith('.msh') or not all(ord(c)<128 for c in path):
            summary['export'] = 'skipped_non_ascii_or_invalid_path'
        elif os.path.isfile(path):
            summary['export'] = 'skipped_existing_file'
        elif not os.path.isdir(os.path.dirname(path)):
            summary['export'] = 'skipped_missing_directory'
        else:
            m.InternalObject.WriteFluentInputFile(path)
            if os.path.isfile(path) and os.path.getsize(path)>0:
                summary['export'] = path
                summary['export_validation']='preliminary_only__Fluent_quality_check_required'
                summary['export_bytes']=os.path.getsize(path)
            else:
                summary['export'] = 'failed'
    else:
        summary['export'] = ('not_requested' if not path else
                             'skipped_missing_explicit_quality_ack' if not job.get('quality_review_acknowledged') else
                             'skipped_critical_quality_gate')
    return summary



def editable_specifications(job, state, for_names_only=False):
    base=float(job.get('base_mm',12.0))
    groups,evidence=editable_group_candidates(state,base)
    boundary=named_boundary_check(state)
    overrides,disabled=local_refinement_rules(job)
    manual=[]
    if not for_names_only:
        for c in Model.Mesh.Children:
            if not utext(c.Name).startswith('AutoMesh_'):
                manual.append(c)
    reserved=set()
    manual_budget=[]
    for c in manual:
        if _is_sizing(c):
            ids=_effective_sizing_ids(c)
            reserved.update(ids)
            manual_budget.append({'category':'additional_manual','group_key':'manual:'+utext(c.Name),
                                  'ids':sorted(ids),'mm':_read_control_mm(c)})
    group_overrides,group_disabled,group_enabled=group_refinement_rules(job,groups)
    group_disabled.update(g.get('group_key') for g in groups
                          if not g.get('default_enabled',True) and g.get('group_key') not in group_enabled and g.get('group_key') not in group_overrides)
    if for_names_only:
        budget={'status':'not_applicable_names_only'}
    else:
        baseline, global_status, global_note = _global_budget_for_local_publication(job)
        budget=evaluate_refinement_budget(state,manual_budget+groups,overrides,disabled,
                                         base_mm=baseline,enforce_global=(baseline is not None),
                                         group_overrides=group_overrides,disabled_groups=group_disabled,
                                         enabled_groups=(group_enabled|set(group_overrides)))
        budget['global_size_status']=global_status
        if global_note:
            budget['warning']=global_note
    occupied=set(reserved)
    for existing_boundary in state.get('named',[]):
        if utext(existing_boundary.get('name','')).lower().strip() in ('inlet','outlet'):
            occupied.update(int(i) for i in existing_boundary.get('ids',[]))
    specs=[]
    def proposal_order(g):
        key=g.get('group_key')
        manual=(g['category']=='named_existing')
        explicit=(key in group_enabled or key in group_overrides)
        return (1 if manual else 0, 0 if explicit else 1,
                float(g.get('area_mm2_min',0)), key or '')
    proposals=sorted(groups,key=proposal_order)
    for index, gr in enumerate(proposals):
        category=gr['category']
        if for_names_only and category=='named_existing':
            continue  # already exists; publish independent AM_* inventory
        if (category in disabled and gr.get('group_key') not in group_enabled and gr.get('group_key') not in group_overrides) or gr.get('group_key') in group_disabled:continue
        ids=sorted(set(int(i) for i in gr['ids'])-occupied)
        if not ids:continue
        if category.startswith('named_'):
            if category == 'named_existing':
                targets=[n for n in state.get('named',[]) if utext(n.get('name','')) == utext(gr.get('region_key','')) and set(n.get('ids',[]))==set(ids)]
            else:
                targets=[n for n in state.get('named',[]) if set(n.get('ids',[]))==set(ids)
                         and utext(n['name']).lower().startswith(category.split('_')[1])]
            if not targets:
                continue
            ns_name=utext(targets[0]['name'])
            suffix=('User_%d' % (index+1) if category == 'named_existing' else
                    'Port_%s_%d' % (category.split('_')[1],index+1))
            auxiliary=False
        else:
            family={'circular_cylindrical':'Hole','narrow_parallel':'Gap',
                    'curved_surface':'Curve', 'area_small':'Area',
                    'area_repeated':'Repeat','area_review':'Review'}.get(category,'Geometry')
            region=utext(gr.get('region_key', 'G%d'%(index+1)))
            region=''.join(c if c.isalnum() or c=='_' else '_' for c in region)[:32]
            suffix='%s_%s' % (family,region)
            legacy_name='AM_'+suffix
            if any(utext(n.get('name',''))==legacy_name for n in state.get('named',[])):
                ns_name=legacy_name
            else:
                family_cn={'Plane':u'平面','Cylinder':u'圆柱面',
                           'AnalyticCurve':u'解析曲面','Freeform':u'自由曲面',
                           'Other':u'其他面','Cylindrical':u'圆柱面'}
                kind_cn={'area_small':u'小面积','area_repeated':u'重复面积',
                         'area_review':u'待审核','circular_cylindrical':u'孔壁',
                         'narrow_parallel':u'窄缝','curved_surface':u'曲面'}
                cn=u'%s%s' % (kind_cn.get(category,u'几何'),
                              family_cn.get(utext(gr.get('surface_family','Other')),u'几何面'))
                ns_name=u'AM_'+cn+u'_'+utext(region)
                suffix=cn+u'_'+utext(region)
            auxiliary=True
        specs.append({'ns_name':ns_name,'sizing_name':'AutoMesh_'+suffix,
                      'auxiliary':auxiliary,'category':category,
                      'ids':ids,'mm':float(group_overrides.get(gr.get('group_key'),overrides.get(category,gr['mm']))),
                      'group_key':gr.get('group_key'),'group':gr})
        occupied.update(ids)
    if len(specs)>40:
        raise RuntimeError('More than 40 ENABLED groups. Disable groups in the list before publishing (all candidates remain available).')
    return specs, {'boundary':boundary,'budget':budget,'geometry_evidence':evidence}


def _named_objects():
    result={}
    for ns in Model.NamedSelections.Children:
        name=utext(ns.Name)
        if name in result:raise RuntimeError('Duplicate Named Selection: '+name)
        result[name]=ns
    return result


def _sizing_objects():
    result={}
    for obj in Model.Mesh.Children:
        name=utext(obj.Name)
        if name in result:raise RuntimeError('Duplicate mesh control: '+name)
        result[name]=obj
    return result


def _ids(obj):
    return set(int(i) for i in obj.Location.Ids)


def _sizing_linked_to(sizing, ns):
    """Verify an actual native Named Selection association (not face-ID fallback)."""
    for field in ('Location', 'NamedSelection'):
        try:
            selected = getattr(sizing, field)
            if selected is ns:
                return True
            if selected is None or utext(getattr(selected, 'Name', '')) != utext(ns.Name):
                continue
            found = set(int(i) for i in selected.Location.Ids)
            expected = set(int(i) for i in ns.Location.Ids)
            if found and found == expected:
                return True
        except Exception:
            pass
    return False

def _bind_named_sizing(control, ns):
    """Bind the existing NS using Mechanical Location API; fail closed."""
    if ns is None:
        raise RuntimeError('AutoMesh: Named Selection object is None')
    expected = _ids(ns)
    if not expected:
        raise RuntimeError('AutoMesh: empty Named Selection: '+utext(ns.Name))
    control.Location = ns
    if not _sizing_linked_to(control, ns):
        raise RuntimeError('AutoMesh: Location Named Selection readback failed: '+utext(ns.Name))
    actual = _effective_sizing_ids(control)
    if actual != expected:
        raise RuntimeError('AutoMesh: Sizing face scope mismatch: '+utext(ns.Name))
def _create_verified_named_sizing(mesh, name, ns, mm):
    """Only create a Sizing with verified editable native NS scoping.

    On failure, delete ONLY the newly-created control. Never delete existing
    controls, named selections or generated mesh. Report the failing property.
    """
    ctrl=mesh.AddSizing()
    stage='Name'
    try:
        ctrl.Name=name
        stage='ElementSize'
        ctrl.ElementSize=Quantity('%g [mm]'%float(mm))
        stage='Location (native Named Selection object)'
        _bind_named_sizing(ctrl,ns)
        stage='ElementSize readback'
        if abs(_read_control_mm(ctrl)-float(mm))>max(0.0001,0.001*float(mm)):
            raise RuntimeError('Size readback differs from requested size')
        return ctrl
    except Exception as exc:
        rollback='not attempted'
        try:
            if callable(getattr(ctrl,'Delete',None)):
                ctrl.Delete()
                rollback='new Sizing deleted'
            else:
                children=getattr(mesh,'Children',None)
                if isinstance(children,list) and ctrl in children:
                    children.remove(ctrl)
                    rollback='new mock Sizing removed'
                else:
                    rollback='new Sizing could not be removed automatically; inspect Mesh tree'
        except Exception as cleanup_error:
            rollback='new Sizing cleanup failed: '+utext(cleanup_error)
        raise RuntimeError('Sizing publication failed for '+utext(ns.Name)+
            ' at '+stage+': '+utext(exc)+'; '+rollback+
            '. Existing mesh, Named Selections and manual Sizing were not changed.')


def publish_named_only(job, state):
    """Create ONLY AutoMesh auxiliary named groups, even if a mesh exists.

    Never clears existing mesh, adds Sizing, or changes global mesh settings.
    Preserves already user-edited groups rather than rewriting their scopes.
    """
    verify_live_boundaries(state)
    specs, evidence=editable_specifications(job,state,for_names_only=True)
    existing=_named_objects()
    all_face_ids=set(int(f['id']) for f in state['faces'])
    for spec in specs:
        if not spec['auxiliary']:continue
        name=spec['ns_name']
        if name in existing:
            current=_ids(existing[name])
            if not current or not current.issubset(all_face_ids):
                raise RuntimeError('Existing named group has invalid geometry: '+name)
            if bool(existing[name].SendToSolver):
                raise RuntimeError('Auxiliary NS has SendToSolver enabled: '+name)
    if not hasattr(Model,'AddNamedSelection'):
        raise RuntimeError('ANSYS ACT Model.AddNamedSelection unavailable; no naming created')
    before=int(Model.Mesh.Elements)
    report=[]
    for spec in specs:
        if not spec['auxiliary']:continue  # inlet/outlet are user-owned, leave untouched
        name=spec['ns_name']
        if name in existing:
            ns=existing[name]; status='preserved'
        else:
            ns,scope_method,scope_note=create_verified_area_selection(spec,state)
            existing[name]=ns
            status='created'
        report.append({'named_selection':name,'category':spec['category'],
                       'group_key':spec.get('group_key'),'status':status,
                       'face_count':len(_ids(ns)),
                       'scope_method':(scope_method if status=='created' else 'preserved'),
                       'scope_note':(scope_note if status=='created' else 'manual scope not changed')})
    after=int(Model.Mesh.Elements)
    if any(x['status']=='created' for x in report):
        manifest_path=os.path.join(os.path.dirname(JOB_FILE),'editable_manifest.json')
        write(manifest_path,{'status':'names_only','project':state['project'],
                             'fingerprint':state['fingerprint'],'prepared':[]})
    return {'created_named_count':sum(x['status']=='created' for x in report),
            'named_groups':report,'mesh_elements_before':before,
            'mesh_elements_after':after,'mesh_preserved':before==after,
            'created_sizing_count':0,'generate_mesh_called':False,
            'global_sizing_changed':False,
            'note':'ANSYS may invalidate an existing mesh after geometry scoping changes; inspect the current Mesh tree. Re-scan before publishing Sizing.'}


def chinese_auxiliary_name(name):
    """Translate only AutoMesh-owned legacy auxiliary IDs, retaining suffix identity."""
    if not utext(name).startswith('AM_'):
        return None
    text=utext(name)
    rules=[('AM_Area_Cylinder_', u'AM_小面积圆柱面_'),
           ('AM_Area_Plane_',u'AM_小面积平面_'),
           ('AM_Area_AnalyticCurve_',u'AM_小面积解析曲面_'),
           ('AM_Repeat_Cylinder_',u'AM_重复圆柱面_'),
           ('AM_Repeat_Plane_',u'AM_重复平面_'),
           ('AM_Repeat_AnalyticCurve_',u'AM_重复解析曲面_'),
           ('AM_Review_',u'AM_待审核_'),('AM_Gap_',u'AM_窄缝_'),
           ('AM_Hole_',u'AM_孔壁_'),('AM_Curve_',u'AM_曲面_')]
    for prefix,new in rules:
        if text.startswith(prefix):
            return new+text[len(prefix):]
    return None


def rename_existing_auxiliary(job,state):
    """Explicit, non-destructive migration of legacy AutoMesh NS names.

    Never touch user-owned names, solver boundaries, or NS bound to Sizing.
    No new NS, no generation, no scoping changes.
    """
    before=int(Model.Mesh.Elements)
    existing=_named_objects()
    items=[]
    occupied=set(existing)
    for old,ns in sorted(existing.items()):
        new=chinese_auxiliary_name(old)
        if not new or new==old:
            continue
        reason=None
        if bool(ns.SendToSolver):
            reason='SendToSolver enabled; not an eligible auxiliary group'
        elif new in occupied:
            reason='Chinese destination name already exists'
        else:
            for sizing in Model.Mesh.Children:
                if _is_sizing(sizing) and _sizing_linked_to(sizing,ns):
                    reason='Sizing already linked; preserve original identity'
                    break
        items.append((old,new,ns,reason))
        occupied.add(new)
    report=[]
    for old,new,ns,reason in items:
        if reason:
            report.append({'old_name':old,'new_name':new,'status':'skipped','reason':reason})
            continue
        try:
            ns.Name=new
            if utext(ns.Name)!=new:
                raise RuntimeError('ANSYS did not preserve Chinese name')
            report.append({'old_name':old,'new_name':new,'status':'renamed'})
        except Exception as exc:
            try:ns.Name=old
            except Exception:pass
            report.append({'old_name':old,'new_name':new,'status':'skipped',
                           'reason':'ANSYS rejected name: '+utext(exc)[:140]})
    return {'renamed_auxiliary':report,
            'renamed_count':sum(x['status']=='renamed' for x in report),
            'skipped_count':sum(x['status']=='skipped' for x in report),
            'mesh_elements_before':before,'mesh_elements_after':int(Model.Mesh.Elements),
            'generate_mesh_called':False,'created_sizing_count':0,
            'note':'Legacy AM_* names only. User names and Sizing-bound names were preserved.'}


def prepare_editable(job, state):
    before_mesh=int(Model.Mesh.Elements)
    if before_mesh>0 and not job.get('allow_existing_mesh_sizing',False):
        raise RuntimeError('Existing mesh: explicit Sizing publication consent required; original mesh will not be cleared by AutoMesh')
    if before_mesh>0 and job.get('apply_global',False):
        raise RuntimeError('Existing mesh: global size overwrite prohibited; disable Apply Global')
    if ('CFD' not in utext(Model.Mesh.PhysicsPreference).upper() or
            'FLUENT' not in utext(Model.Mesh.SolverPreference).upper()):
        raise RuntimeError('Prepare requires CFD + Fluent Meshing settings; no automatic change')
    verify_live_boundaries(state)
    specs, ev=editable_specifications(job,state)
    named=_named_objects();sizing=_sizing_objects()
    expected_control_names=set(x['sizing_name'] for x in specs)
    legacy=sorted(name for name in sizing if name.startswith('AutoMesh_')
                  and name not in expected_control_names)
    if legacy:
        raise RuntimeError('Existing legacy/unmatched AutoMesh controls are present: %s. Inspect in Meshing first; no automatic deletion or duplicate sizing.' % ', '.join(legacy[:8]))
    all_face_ids=set(int(f['id']) for f in state['faces'])
    manual_changed=[]
    for spec in specs:
        nname=spec['ns_name'];cname=spec['sizing_name']
        if nname in named:
            ns=named[nname]
            current=_ids(ns)
            if not current or not current.issubset(all_face_ids):
                raise RuntimeError('Existing named group has invalid geometry: '+nname)
            if spec['auxiliary'] and current!=set(spec['ids']):
                manual_changed.append(nname)
        if cname in sizing and nname not in named:
            raise RuntimeError('Existing sizing without expected named selection: '+cname)
        if cname in sizing and not _sizing_linked_to(sizing[cname],named[nname]):
            raise RuntimeError('Existing sizing is not bound to expected NS: '+cname)
    if not hasattr(Model,'AddNamedSelection'):
        raise RuntimeError('ANSYS ACT API Model.AddNamedSelection unavailable; no controls created')
    if not hasattr(Model.Mesh,'AddSizing'):
        raise RuntimeError('ANSYS ACT API Mesh.AddSizing unavailable; no controls created')
    if 'GeometryDefineByType' not in globals() or not hasattr(GeometryDefineByType,'Component'):
        raise RuntimeError('ANSYS ACT geometry Component scoping enum unavailable; no controls created')
    manifest_path=os.path.join(os.path.dirname(JOB_FILE),'editable_manifest.json')
    # Keep the prior successful manifest until this prepare finishes.
    report=[]
    for spec in specs:
        nname=spec['ns_name'];cname=spec['sizing_name']
        if nname in named:
            ns=named[nname];ns_status='preserved'
        else:
            if not spec['auxiliary']:
                raise RuntimeError('Manual inlet/outlet disappeared; scan again')
            ns,scope_method,scope_note=create_verified_area_selection(spec,state)
            named[nname]=ns;ns_status='created'
        if cname in sizing:
            ctrl=sizing[cname];sz_status='preserved'
        else:
            ctrl=_create_verified_named_sizing(Model.Mesh,cname,ns,spec['mm'])
            sz_status='created' 
        report.append({'named_selection':nname,'sizing':cname,'category':spec['category'],
                      'group_key':spec.get('group_key'),'named_status':ns_status,'sizing_status':sz_status,
                      'face_count':len(_ids(ns)),'size_mm':_read_control_mm(ctrl),
                      'user_scope_changed':nname in manual_changed,
                      'scope_method':(scope_method if ns_status=='created' else 'preserved'),
                      'scope_note':(scope_note if ns_status=='created' else 'manual scope not changed')})
    global_changed=False
    if job.get('apply_global',False):
        base=float(job.get('base_mm',12.0))
        Model.Mesh.ElementSize=Quantity('%g [mm]'%base)
        Model.Mesh.MaximumSize=Quantity('%g [mm]'%(2.0*base))
        Model.Mesh.GrowthRate=1.3
        global_changed=True
    spec_by_name={sp['sizing_name']:sp for sp in specs}
    manifest={'status':'ready','project':state['project'],'fingerprint':state['fingerprint'],
              'global_applied':global_changed,'base_mm':float(job.get('base_mm',12.0)),
              'prepared':[{'ns':x['named_selection'],'sizing':x['sizing'],
                'category':spec_by_name[x['sizing']]['category'],
                'group_key':spec_by_name[x['sizing']].get('group_key'),
                'gap_mm_min':spec_by_name[x['sizing']]['group'].get('gap_mm_min')}
                for x in report]}
    write(manifest_path,manifest)
    return {'editable_named_groups':report,'created_named_count':sum(x['named_status']=='created' for x in report),
            'created_sizing_count':sum(x['sizing_status']=='created' for x in report),
            'preserved_manual_groups':manual_changed,'global_sizing_changed':global_changed,'geometry_evidence':ev['geometry_evidence'],
            'resource_preflight':ev['budget'],'generate_mesh_called':False,
            'mesh_elements_before':before_mesh,'mesh_elements_after':int(Model.Mesh.Elements),
            'mesh_preserved':int(Model.Mesh.Elements)==before_mesh,
            'existing_mesh_warning':('ANSYS might invalidate an existing mesh after adding Sizing; AutoMesh did not clear or regenerate it' if before_mesh>0 else ''),
            'instructions':'Inspect and edit AM_* Named Selections and AutoMesh_* Sizing in Meshing; then select Generate Reviewed Mesh.'}



def existing_named_sizing_name(name):
    """Stable ACT-safe identifier. Never rename or rewrite a user-owned selection."""
    digest=hashlib.sha1(utext(name).encode('utf-8')).hexdigest()[:10]
    ascii_part=''.join(ch if ch.isalnum() and ord(ch)<128 else '_' for ch in utext(name))
    ascii_part=ascii_part.strip('_')[:22] or 'Group'
    return 'AutoMesh_NS_%s_%s' % (ascii_part,digest)


def validate_existing_named_specs(job,state,allow_empty=False):
    """Reuse exactly the CURRENT Meshing Named Selection scopes. No geometry regrouping.

    All requested names and all pre-existing controls are validated before any write.
    The task explicitly lists which names are opted-in and at what exact mm size.
    """
    plan=job.get('plan') or {}
    requested=plan.get('existing_named_sizes') or {}
    old_scopes=plan.get('existing_named_scope_ids') or {}
    if not isinstance(requested,dict) or (not requested and not allow_empty):
        raise RuntimeError('No Named Selection selected: choose one or more groups and mm size first')
    if len(requested)>40:
        raise RuntimeError('More than 40 existing Named Selections requested. Select a smaller batch')
    named=_named_objects()
    sizing=_sizing_objects()
    current_faces={int(f['id']) for f in state['faces']}
    specs=[]; used=set()
    missing=[]
    for name in sorted(requested):
        if not isinstance(name,(str,type(u''))):
            raise RuntimeError('Invalid Named Selection name')
        if utext(name).strip().lower() in ('inlet','outlet'):
            raise RuntimeError('inlet/outlet are solver boundaries and excluded from automatic local sizing')
        ns=named.get(name)
        original_name=name
        if ns is None:
            expected=set(int(i) for i in old_scopes.get(name,[]) if i is not None)
            matches=[]
            if expected:
                for live_name, candidate in named.items():
                    if utext(live_name).strip().lower() in ('inlet','outlet'):
                        continue
                    try:
                        if _ids(candidate)==expected:
                            matches.append((live_name,candidate))
                    except Exception:
                        pass
            if len(matches)==1:
                name,ns=matches[0]
            else:
                if len(matches)>1:
                    hint='same-scope candidates: '+', '.join(utext(m[0]) for m in matches[:8])
                elif expected:
                    hint='no identical current face scope (previous %d faces)'%len(expected)
                else:
                    hint='old scope unavailable; click read-only scan to refresh'
                missing.append(utext(original_name)+' — '+hint)
                continue
        ids=_ids(ns)
        if not ids or not ids.issubset(current_faces):
            raise RuntimeError('Named Selection has no valid geometric face scope: '+utext(name))
        # faces after a CAD/model change or user edit. A published sizing must
        # never silently switch to that different scope using stale settings.
        # Re-scanning records the current scope as a new authoritative draft.
        expected_scope=set(int(i) for i in old_scopes.get(original_name,[]) if i is not None)
        if expected_scope and ids != expected_scope:
            raise RuntimeError('命名组作用面已变化（尚未创建 Sizing）：'+utext(original_name)+
                 '，旧 %d 面，当前 %d 面；请只读扫描并重新审核。'%(len(expected_scope),len(ids)))
        if used.intersection(ids):
            raise RuntimeError('Selected Named Selection scopes overlap: '+utext(name)+'; choose non-overlapping groups')
        used.update(ids)
        if utext(name).startswith('AM_') and bool(ns.SendToSolver):
            raise RuntimeError('AutoMesh auxiliary Named Selection is set SendToSolver=True: '+utext(name))
        try:
            # The user's setting is keyed by the originally submitted name.
            # An exact-scope rename changes `name`, not that input key.
            mm=float(requested[original_name])
        except Exception:
            raise RuntimeError('Invalid local size for '+utext(name))
        if math.isnan(mm) or math.isinf(mm) or not 0.35<=mm<=500:
            raise RuntimeError('Local size must be 0.35-500 mm: '+utext(name))
        sizing_name=existing_named_sizing_name(name)
        existing=sizing.get(sizing_name)
        if existing is not None and not _sizing_linked_to(existing,ns):
            raise RuntimeError('Existing AutoMesh Sizing is linked to another Named Selection: '+sizing_name)
        # Avoid double-scoping a selected NS if a legacy AutoMesh or a user
        # already provided a Sizing on it. Preserve its actual name and mm.
        linked=[]
        for obj in Model.Mesh.Children:
            if _is_sizing(obj) and _sizing_linked_to(obj,ns):
                linked.append(obj)
        if len(linked)>1:
            raise RuntimeError('More than one existing Sizing is bound to '+utext(name)+'; resolve duplicates in Meshing')
        if linked:
            if existing is not None and existing is not linked[0]:
                raise RuntimeError('Conflicting Sizing identity on '+utext(name))
            existing=linked[0]
            sizing_name=utext(existing.Name)
        specs.append({'ns_name':name,'ns':ns,'ids':sorted(ids),'requested_mm':mm,
                      'sizing_name':sizing_name,'existing_sizing':existing,'remapped_from':(original_name if original_name!=name else '')})
    if missing:
        raise RuntimeError('命名组校验未通过（尚未创建任何 Sizing）：\n'+
             '\n'.join(missing[:20])+ '\n请在 AutoMesh 刷新命名；如有重名/相同作用域组，请先在 Meshing 核实。')
    return specs


def publish_existing_named_sizing(job,state):
    if 'CFD' not in utext(Model.Mesh.PhysicsPreference).upper() or 'FLUENT' not in utext(Model.Mesh.SolverPreference).upper():
        raise RuntimeError('Require CFD + Fluent')
    bd=verify_live_boundaries(state)
    n0=int(Model.Mesh.Elements)
    if n0 and not job.get('allow_existing_mesh_sizing',False):
        raise RuntimeError('Existing mesh: consent required')
    if job.get('apply_global',False):raise RuntimeError('Global sizing prohibited')
    path=os.path.join(os.path.dirname(JOB_FILE),'editable_manifest.json')
    prior=read(path) if os.path.isfile(path) else None
    previous_owned=set()
    if (isinstance(prior,dict) and prior.get('status')=='ready' and
            prior.get('project')==state['project'] and
            prior.get('fingerprint')==state['fingerprint']):
        for item in prior.get('prepared',[]):
            if isinstance(item,dict) and 'ns' in item and 'sizing' in item and utext(item['sizing'])==existing_named_sizing_name(utext(item['ns'])):
                previous_owned.add((utext(item['ns']),utext(item['sizing'])))
        # A previously disabled managed control may still be present in the
        # current Mesh tree. Carry retirement forward until manually resolved.
        for item in prior.get('retired_managed',[]):
            if isinstance(item,dict) and 'ns' in item and 'sizing' in item and utext(item['sizing'])==existing_named_sizing_name(utext(item['ns'])):
                previous_owned.add((utext(item['ns']),utext(item['sizing'])))
    if not (job.get('plan') or {}).get('existing_named_sizes') and not previous_owned:
        raise RuntimeError('No selected groups and no verified AutoMesh-owned Sizing to disable')
    specs=validate_existing_named_specs(job,state,allow_empty=bool(previous_owned))
    owned={}
    for x in specs:
        c=x['existing_sizing']
        if (c is not None and x['sizing_name']==existing_named_sizing_name(x['ns_name'])
                and (utext(x['ns_name']),utext(c.Name)) in previous_owned):
            owned[c.Name]=x
    requested_pairs = set((utext(x['ns_name']),utext(x['sizing_name'])) for x in specs)
    retired = [{'ns':ns,'sizing':cname} for ns,cname in sorted(previous_owned-requested_pairs)]
    items=[]
    retired_names=set(item['sizing'] for item in retired)
    for c in Model.Mesh.Children:
        if not _is_sizing(c) or _is_control_suppressed(c) or utext(c.Name) in retired_names:
            continue
        x=owned.get(c.Name)
        items.append({'category':'additional_manual','group_key':'manual:'+utext(c.Name),
                             'ids':sorted(_effective_sizing_ids(c)),
                             'mm':x['requested_mm'] if x else _read_control_mm(c)})
    for x in specs:
        if x['existing_sizing'] is None:
            items.append({'category':'named_existing','group_key':'existing:'+x['ns_name'],
                                 'ids':x['ids'],'mm':x['requested_mm']})
    # Only controls proven owned by the prior manifest can be suppressed.
    # Validate all of them BEFORE mutating any live controls. Never suppress
    # an unrelated user Sizing even if it has an AutoMesh-looking name.
    live_named=_named_objects()
    live_sizing=_sizing_objects()
    retired_actions=[]
    for item in retired:
        c=live_sizing.get(item['sizing'])
        if c is None:
            item['status']='missing_already'
            continue
        ns=live_named.get(item['ns'])
        if not _is_sizing(c) or ns is None or not _sizing_linked_to(c,ns):
            raise RuntimeError('Cannot safely deactivate changed AutoMesh-owned control: '+item['sizing']+
                               '; inspect in Meshing. No existing control was modified.')
        if not hasattr(c,'Suppressed'):
            raise RuntimeError('This Mechanical Sizing does not expose Suppressed: '+item['sizing']+
                               '; no changes were applied')
        retired_actions.append((c,item))
    base,global_status,global_note=_global_budget_for_local_publication(job)
    budget=evaluate_refinement_budget(state,items,base_mm=base,enforce_global=(base is not None))
    budget['global_size_status']=global_status
    if global_note:
        budget['warning']=global_note
    previous=[];added=[];rows=[];suppression_changes=[]
    # Keep previous known-good manifest unchanged until publication succeeds.
    try:
        for i,x in enumerate(specs):
            ns=x['ns'];c=x['existing_sizing'];want=x['requested_mm']
            if c is None:
                c=_create_verified_named_sizing(Model.Mesh,x['sizing_name'],ns,want)
                added.append(c);st='created'
            elif c.Name in owned:
                if _is_control_suppressed(c):
                    suppression_changes.append((c,True))
                    c.Suppressed=False
                    if _is_control_suppressed(c):
                        raise RuntimeError('Cannot reactivate owned Sizing: '+utext(c.Name))
                old=_read_control_mm(c)
                if abs(old-want)>max(0.0001,0.001*want):
                    previous.append((c,old))
                    c.ElementSize=Quantity('%g [mm]'%want)
                    if abs(_read_control_mm(c)-want)>max(0.0001,0.001*want):raise RuntimeError('Size mismatch: '+utext(c.Name))
                    st='updated'
                else:st='unchanged'
            else:st='preserved_manual_unmanaged'
            if not _sizing_linked_to(c,ns) or _effective_sizing_ids(c)!=_ids(ns):
                raise RuntimeError('Scope mismatch: '+utext(c.Name))
            rows.append({'named_selection':x['ns_name'],'sizing':utext(c.Name),
                           'sizing_status':st,'size_mm':_read_control_mm(c),
                           'remapped_from':x.get('remapped_from','')})
        # Deactivate only verified owned controls no longer selected; keep
        # controls inspectable in the Meshing tree instead of deleting them.
        for c,item in retired_actions:
            previous_status=_is_control_suppressed(c)
            if not previous_status:
                suppression_changes.append((c,previous_status))
                c.Suppressed=True
                if not _is_control_suppressed(c):
                    raise RuntimeError('Suppressed readback mismatch: '+utext(c.Name))
            item['status']='suppressed_verified'
        write(path,{'status':'ready','project':state['project'],'fingerprint':state['fingerprint'],
                    'global_applied':False,'base_mm':base,'prepared':[
            {'ns':r['named_selection'],'sizing':r['sizing']} for r in rows
            if r['sizing_status']!='preserved_manual_unmanaged' and
               r['sizing']==existing_named_sizing_name(r['named_selection'])],
            'retired_managed':retired})
    except Exception as exc:
        errors=[]
        for c,prior_suppressed in reversed(suppression_changes):
            try:
                c.Suppressed=prior_suppressed
                if _is_control_suppressed(c)!=prior_suppressed:
                    raise RuntimeError('suppression restore readback mismatch')
            except Exception as e:errors.append('restore Suppressed '+utext(c.Name)+': '+utext(e))
        for c,old in reversed(previous):
            try:c.ElementSize=Quantity('%g [mm]'%old)
            except Exception as e:errors.append('restore '+utext(c.Name)+': '+utext(e))
        for c in reversed(added):
            try:c.Delete()
            except Exception as e:errors.append('delete '+utext(c.Name)+': '+utext(e))
        raise RuntimeError('Republish: '+utext(exc)+'; rollback='+('; '.join(errors) if errors else 'ok'))
    return {'existing_named_sizing':rows,'created_named_count':0,
            'created_sizing_count':sum(r['sizing_status']=='created' for r in rows),
            'updated_sizing_count':sum(r['sizing_status']=='updated' for r in rows),
            'resource_preflight':budget,'boundary':bd,'global_sizing_changed':False,
            'retired_managed':retired,
            'suppressed_sizing_count':sum(x.get('status')=='suppressed_verified' for x in retired),
            'warning':('AutoMesh-owned unselected Sizing suppressed and retained in Meshing tree. Inspect changes before generating' if retired else ''),
            'generate_mesh_called':False,'mesh_preserved':n0==int(Model.Mesh.Elements)}
#                 
def _mesh_control_digest():
    info=[]
    for c in Model.Mesh.Children:
        if _is_sizing(c):
            # Sizing.Location may be a NamedSelection, not an ISelectionInfo.
            # Never replace an unreadable scope with an empty list in a
            # fingerprint: that could hide a scope change.
            scope=sorted(_effective_sizing_ids(c))
        else:
            try:scope=sorted(int(x) for x in c.Location.Ids)
            except Exception:scope=['not-applicable']
        try:size=utext(c.ElementSize)
        except Exception:size='not-applicable'
        info.append((utext(c.Name),utext(c.DataModelObjectCategory),scope,size))
    for key in ('ElementSize','MaximumSize','GrowthRate','PhysicsPreference',
                'SolverPreference','ElementOrder','CaptureCurvature','CaptureProximity'):
        try:info.append((key,utext(getattr(Model.Mesh,key))))
        except Exception:info.append((key,'unreadable'))
    return hashlib.sha1(json.dumps(sorted(info,key=lambda x:utext(x[0])),
                     ensure_ascii=True,sort_keys=True).encode('utf-8')).hexdigest()


def mesh_reference_status(state):
    marker=os.path.join(os.path.dirname(JOB_FILE),'automesh_generation_reference.json')
    result={'status':'unverified','note':'Existing element count is not proof of mesh freshness'}
    try:result['act_object_state']=utext(Model.Mesh.ObjectState)
    except Exception:result['act_object_state']='unavailable'
    if not os.path.isfile(marker):return result
    try:
        previous=read(marker)
        if previous.get('project')!=state['project'] or previous.get('fingerprint')!=state['fingerprint']:
            result['status']='model_different';return result
        if previous.get('digest_version')!=2:
            result['status']='reference_version_unknown'
            result['note']='Legacy fingerprint cannot be compared with this version; refresh only after a newly confirmed generation'
            return result
        result['status']=('controls_match_last_automesh_generation' if
            previous.get('digest')==_mesh_control_digest() else 'controls_changed_after_automesh_generation')
        result['reference_time']=previous.get('timestamp')
        result['note']='Historical AutoMesh signature only. It cannot certify current meshing data or external changes.'
    except Exception as e:result['note']='Reference read failed: '+utext(e)[:120]
    return result


def confirm_remesh_in_meshing(old_count):
    # Fail closed if a real user confirmation cannot be displayed.
    try:
        import clr
        clr.AddReference('System.Windows.Forms')
        from System.Windows.Forms import (MessageBox,MessageBoxButtons,MessageBoxIcon,
                                          MessageBoxDefaultButton,DialogResult)
        msg=(u'当前存在 %d 个旧网格单元。\n\n重新生成将先调用 ClearGeneratedData()，'
             u'清除当前已有网格，再按现有 Sizing 生成。'
             u'如果新网格失败，旧网格不会自动恢复。\n\n'
             u'请先在 Workbench 保存工程及备份。是否确认清除旧网格并重新生成？' % old_count)
        choice=MessageBox.Show(msg,u'AutoMesh — 重新划分确认',MessageBoxButtons.YesNo,
                 MessageBoxIcon.Warning,MessageBoxDefaultButton.Button2)
        return choice==DialogResult.Yes
    except Exception as e:
        raise RuntimeError('Cannot obtain interactive remesh confirmation; no mesh deleted: '+utext(e))


def generate_reviewed(job, state):
    existing_count=int(Model.Mesh.Elements)
    if ('CFD' not in utext(Model.Mesh.PhysicsPreference).upper() or
            'FLUENT' not in utext(Model.Mesh.SolverPreference).upper()):
        raise RuntimeError('Reviewed generation requires CFD + Fluent settings')
    boundary=verify_live_boundaries(state)  # re-read NOW, after operator Meshing edits
    manifest_path=os.path.join(os.path.dirname(JOB_FILE),'editable_manifest.json')
    qr_manifest_path=os.path.join(os.path.dirname(JOB_FILE),'warning_refinement_managed.json')
    qr_manifest=_current_warning_owner(state) if os.path.isfile(qr_manifest_path) else None
    if not os.path.isfile(manifest_path):
        # Allow verified, explicitly published quality-hotspot SOI controls as
        # the only local sizing; no auto creation or blind remesh.
        if qr_manifest is None:
            raise RuntimeError('No editable control manifest; publish selections and sizing first')
        manifest={'status':'ready','project':state['project'],
                  'fingerprint':state['fingerprint'],'prepared':[]}
    else:
        manifest=read(manifest_path)
    if (manifest.get('status')!='ready' or manifest.get('project')!=state['project'] or
            manifest.get('fingerprint')!=state['fingerprint']):
        raise RuntimeError('Geometry changed since named groups were prepared; scan and review again')
    named=_named_objects();sizing=_sizing_objects()
    unresolved=[]
    for retired in manifest.get('retired_managed',[]):
        c=sizing.get(retired.get('sizing'))
        if c is not None and not _is_control_suppressed(c):
            unresolved.append(utext(c.Name))
    if unresolved:
        raise RuntimeError('Disabled AutoMesh Sizing remains active in Meshing: '+', '.join(unresolved[:12])+'. Manually suppress/delete these old controls, then rescan/review before GenerateMesh. No mesh was cleared.')
    valid_ids=set(int(f['id']) for f in state['faces'])
    areas={int(f['id']):f for f in state['faces']}
    active=[]
    used_prepared=set()
    for entry in manifest['prepared']:
        ns=named.get(entry['ns']);ctrl=sizing.get(entry['sizing'])
        if ns is None or ctrl is None or not _sizing_linked_to(ctrl,ns):
            raise RuntimeError('Prepared named sizing pair removed/unbound: '+entry['sizing'])
        if _is_control_suppressed(ctrl):
            raise RuntimeError('Selected Sizing is suppressed: '+entry['sizing']+'; enable it or republish your intended active groups')
        face_ids=_ids(ns)
        if not face_ids or not face_ids.issubset(valid_ids):
            raise RuntimeError('Reviewed NS contains invalid faces: '+entry['ns'])
        if used_prepared.intersection(face_ids):
            raise RuntimeError('Reviewed named selections overlap; separate their scopes before generation: '+entry['ns'])
        used_prepared.update(face_ids)
        if entry['ns'].startswith('AM_') and bool(ns.SendToSolver):
            raise RuntimeError('Auxiliary NS would reach Fluent; turn off Send to Solver: '+entry['ns'])
        mm=_read_control_mm(ctrl)
        if not 0.35<=mm<=500:
            raise RuntimeError('Manual Sizing exceeds safe range: '+entry['sizing'])
        active.append({'category':entry.get('category',entry['ns']), 'gap_mm_min':entry.get('gap_mm_min'),
                       'ids':sorted(face_ids),'mm':mm})
    # Include any additional operator-created Sizing in the conservative
    # surface budget. Unknown controls are flagged explicitly, never counted
    # as an exact RAM prediction.
    known=set(x['sizing'] for x in manifest['prepared'])
    qr_allowed={}
    if qr_manifest is not None:
        qr_allowed=dict((x['name'],x) for x in qr_manifest.get('controls',[]))
        if not qr_allowed or len(qr_allowed)!=len(qr_manifest.get('controls',[])):
            raise RuntimeError('Warning hotspot manifest is missing or duplicated')
    qr_controls=[];qr_proxy=0.0
    unchecked=[]
    for other in Model.Mesh.Children:
        name=utext(other.Name)
        if name in known:continue
        if name.startswith('AutoMesh_QR_'):
            entry=qr_allowed.get(name)
            if entry is None or not _is_sizing(other) or int(other.ObjectId)!=int(entry['object_id']):
                raise RuntimeError('Unverified quality-hotspot Sizing ownership; no mesh cleared: '+name)
            if _is_control_suppressed(other):
                raise RuntimeError('Reviewed quality-hotspot Sizing is suppressed: '+name)
            if utext(other.Type)!=utext(SizingType.SphereOfInfluence):
                raise RuntimeError('Quality-hotspot sizing type was changed; review manually: '+name)
            if _effective_sizing_ids(other)!=set([int(entry['body_id'])]):
                raise RuntimeError('Quality-hotspot body scope changed: '+name)
            if int(other.SphereCenter.ObjectId)!=int(entry['cs_object_id']):
                raise RuntimeError('Quality-hotspot sphere center changed: '+name)
            mm=_read_control_mm(other)
            # SphereRadius is a Quantity; use the same strict unit parser as ElementSize.
            import re
            radius_txt=utext(other.SphereRadius).strip()
            match=re.search(r'([-+]?\d+(?:\.\d+)?(?:[Ee][-+]?\d+)?)\s*\[?\s*(mm|cm|m|um|in|inch)\b',radius_txt,re.I)
            if match is None:
                raise RuntimeError('Cannot read quality-hotspot sphere radius and unit: '+name)
            radius=float(match.group(1))*unit_to_mm(match.group(2))
            if not 0.5<=mm<=5.0 or not 1.5<=radius<=MAX_RADIUS_MM:
                raise RuntimeError('Quality-hotspot sphere size edited outside conservative review range: '+name)
            qr_proxy+=(4.0/3.0)*math.pi*(radius/mm)**3
            if qr_proxy>MAX_CELL_PROXY:
                raise RuntimeError('Quality-hotspot cumulative refinement proxy exceeds 32GB conservative limit')
            qr_controls.append(name)
            continue
        if _is_sizing(other) and _is_control_suppressed(other):
            continue
        if 'sizing' not in utext(other.DataModelObjectCategory).lower():
            unchecked.append(utext(other.Name))
            continue
        try:
            other_ids=_effective_sizing_ids(other)
            active.append({'category':'additional_manual','ids':sorted(other_ids),
                           'mm':_read_control_mm(other)})
        except Exception as exc:
            raise RuntimeError('Cannot assess manually added sizing %s: %s' % (utext(other.Name),utext(exc)))
    if qr_allowed and set(qr_controls)!=set(qr_allowed):
        raise RuntimeError('Previously published quality-hotspot control missing; no mesh cleared')
    # Re-evaluate operator-edited sizes AND the current effective global size.
    # Reviewed *generation* is destructive after approval; unlike local
    # publication, it cannot use an unknown/automatic global size for the
    # 32 GiB volume guard. Reject BEFORE ClearGeneratedData/GenerateMesh.
    try:
        baseline=_global_meshing_size_mm(job,False)
    except RuntimeError as exc:
        raise RuntimeError('审核生成前无法确认 Meshing 全局网格尺寸（可能显示 0.0 / 自动）。'
                           '请先在当前 Meshing 明确设置正数 Element Size（mm）并重新检查；'
                           '旧网格和控制没有清除或修改。原因：'+utext(exc))
    budget=evaluate_refinement_budget(state,active,base_mm=baseline,enforce_global=True)
    if not active and not qr_controls:raise RuntimeError('No reviewed local controls to generate')
    if existing_count>0:
        emit_progress(None,'等待人工确认','已有网格，需明确确认清除旧网格并重新划分')
        if not confirm_remesh_in_meshing(existing_count):
            raise RuntimeError('Operator cancelled remeshing; existing mesh preserved')
        if int(Model.Mesh.Elements)!=existing_count:
            raise RuntimeError('Mesh changed during confirmation; refused clearing')
        if not callable(getattr(Model.Mesh,'ClearGeneratedData',None)):
            raise RuntimeError('ClearGeneratedData unavailable; existing mesh preserved')
        emit_progress(None,'清理旧网格','用户确认后执行 ClearGeneratedData；不删除 Named Selections 或 Sizing')
        Model.Mesh.ClearGeneratedData()
        if int(Model.Mesh.Elements)>0:
            raise RuntimeError('ClearGeneratedData left old elements; refused GenerateMesh')
    write(RESULT_FILE,{'job_id':job['job_id'],'phase':'generate_reviewed',
                       'status':'in_progress','stage':'GenerateMesh',
                       'summary':{'reviewed_groups':len(active)}})
    started=time.time()
    emit_progress(None,'ANSYS 正在生成网格',
                  'Meshing GenerateMesh() 正在执行；原生 API 未提供可靠百分比，禁止重复提交')
    Model.Mesh.GenerateMesh()
    emit_progress(90,'网格已生成','正在读取单元数量和质量统计')
    if int(Model.Mesh.Elements)<=0:raise RuntimeError('GenerateMesh returned no elements')
    reference_note='Control fingerprint recorded'
    try:
        digest=_mesh_control_digest()
        write(os.path.join(os.path.dirname(JOB_FILE),'automesh_generation_reference.json'),
              {'project':state['project'],'fingerprint':state['fingerprint'],
               'digest':digest,'digest_version':2,
               'timestamp':time.strftime('%Y-%m-%d %H:%M:%S'),
               'mesh_elements':int(Model.Mesh.Elements)})
    except Exception as e:
        reference_note='Generation succeeded; fingerprint unavailable: '+utext(e)[:160]
    quality=assess_quality()
    verdict=reviewed_quality_gate(quality)
    qr_comparison=None
    if qr_controls:
        new_ws=quality_worksheet_summary()
        new_warning=None
        for metric in new_ws.get('metrics',[]):
            if metric.get('name')=='Skewness':new_warning=metric.get('warning_count')
        qr_comparison={'baseline_warning_count':qr_manifest.get('baseline_warning_count'),
                       'new_warning_count':new_warning,
                       'same_worksheet_thresholds_verified':False,
                       'note':'New count is official if returned; worksheet thresholds may change. A smaller count does not by itself prove CFD convergence.'}
        # The previous warning *element IDs* must never be offered again after
        # a successful remesh. Keep the marker file as a readable audit record.
        marker_path=os.path.join(os.path.dirname(JOB_FILE),'warning_refinement_marker.json')
        if os.path.isfile(marker_path):
            marker=read(marker_path);marker['status']='invalid_after_remesh';write(marker_path,marker)
    return {'reviewed_groups':len(active),'resource_preflight':budget,
            'warning_hotspot_controls':qr_controls,'warning_refinement_comparison':qr_comparison,
            'seconds':round(time.time()-started,2),'mesh_elements':int(Model.Mesh.Elements),
            'quality':quality,'quality_status':verdict['status'],
            'quality_gate':verdict['quality_gate'],'next_action':verdict['reason'],
            'boundary':boundary, 'manual_sizing_preserved':True,
            'unestimated_non_sizing_controls':unchecked,
            'reference_note':reference_note,
            'geometry_fingerprint':state['fingerprint'],'project':state['project'],
            'warning':'Preliminary ACT status only; Fluent verification is mandatory'}


# Atomic once-per-job claim. On crash the claim remains, preventing an
# unintentional second GenerateMesh after restart/duplicate SendCommand.
job = read(JOB_FILE)
jobid = utext(job.get('job_id',''))
if len(jobid) != 32 or any(c not in '0123456789abcdef' for c in jobid.lower()):
    raise RuntimeError('Malformed job id; refusing ACT execution')
claims = os.path.join(os.path.dirname(JOB_FILE),'claims')
if not os.path.isdir(claims):
    try:
        os.makedirs(claims)
    except OSError:
        pass
claim_path = os.path.join(claims,jobid+'.claimed')
try:
    fd = os.open(claim_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL)
except OSError:
    raise RuntimeError('This exact job already started; refuses duplicate task')
os.write(fd, str(time.time()).encode('ascii'))
os.close(fd)
result = {'job_id': job.get('job_id'), 'phase': job.get('phase'), 'status': 'started',
          'stage': 'Meshing ACT command accepted',
          'timestamp': time.strftime('%Y-%m-%d %H:%M:%S')}
write(RESULT_FILE, result)
emit_progress(3,'已进入 Meshing','ACT 命令被当前会话接受，开始核验几何与工程')
try:
    cancellation_checkpoint()
    phase = job.get('phase')
    # A failed readiness check must preserve the last successful snapshot.
    # Guard all phases, not just inspect, before touching live Meshing state.
    Model = resolve_current_model(globals().get('Model'), ExtAPI)
    if phase in ('inspect','auto_onepass','generate_reviewed','export','publish_existing_sizing'):
        state=snapshot()
    elif phase in ('quality_check','diagnose','mark_failed','mark_warning','mark_high_skew','warning_refine_preview','warning_refine_publish'):
        state=read_only_verified_snapshot(job)
    else:
        state=verify_previous_snapshot(job)
    cancellation_checkpoint()
    if phase in ('inspect','auto_onepass'):
        write(SNAPSHOT_FILE, state)
    if phase == 'inspect':
        result['summary'] = {'project': state['project'], 'body_count': len(state['bodies']),
                             'face_count': len(state['faces']), 'named_count': len(state['named']),
                             'control_count': len(state['controls']), 'geometry_unit': state['geometry_unit'],
                             'display_unit_system': state['display_unit_system'],
                             'display_length_unit': state['display_length_unit'],
                             'mesh': state['mesh'], 'capabilities':state['capabilities']}
        try:
            emit_progress(67,'按面积分组','正在按面积排序，并结合面型归并相似面')
            groups, ev = editable_group_candidates(state, float(job.get('base_mm', 12.0)))
            emit_progress(92,'整理候选组','分组完成：%d 组；正在写入可编辑清单' % len(groups))
            result['summary']['geometry_evidence'] = ev
            result['summary']['proposed_sizing'] = [{'category':x['category'],
                 'size_mm':x['mm'],'group_key':x.get('group_key'),
                 'size_mm_internal':x['mm'],'face_count':len(x['ids']), 'confidence':x['confidence'],
                 'diameter_mm_min':x.get('diameter_mm_min'),
                 'diameter_mm_max':x.get('diameter_mm_max'),
                 'diameter_band':x.get('diameter_band'), 'region_key':x.get('region_key'),
                 'surface_family':x.get('surface_family'),
                 'area_mm2_min':x.get('area_mm2_min'), 'area_mm2_max':x.get('area_mm2_max'),
                 'face_ids_sample':x['ids'][:12], 'reason':x['reason']}
                 for x in groups]
        except Exception as e:
            result['summary']['geometry_analysis_warning'] = utext(e)
    if phase == 'auto_onepass':
        # Single dispatch: scan, fingerprint, native sizing and generate in one session.
        # A mid-run geometry change is rejected before modifying any controls.
        quick = quick_geometry_identity()
        if quick['project'] != state['project'] or quick['fingerprint'] != state['fingerprint']:
            raise RuntimeError('Geometry changed during one-pass analysis; no mesh modification permitted')
        result['summary']=automatic_mesh(job,state)
    elif phase != 'inspect':
        cancellation_checkpoint()
        ensure_current(job, state)
        if phase == 'rename_auxiliary':
            emit_progress(68,'中文化辅助命名','仅更改未绑定 Sizing 的旧版 AM_* 组')
            result['summary']=rename_existing_auxiliary(job,state)
        elif phase == 'publish_existing_sizing':
            emit_progress(65,'核验现有命名','只使用已存在的 Named Selection，核对尺寸和安全预算')
            result['summary']=publish_existing_named_sizing(job,state)
        elif phase == 'publish_names':
            emit_progress(72,'发布命名','正在核对原有 Named Selection；不会生成网格')
            result['summary']=publish_named_only(job,state)
        elif phase == 'prepare':
            emit_progress(72,'发布 Sizing','正在核对局部尺寸和作用面；不会清理网格')
            result['summary']=prepare_editable(job,state)
        elif phase == 'generate_reviewed':
            cancellation_checkpoint()
            result['summary']=generate_reviewed(job,state)
        elif phase == 'apply':
            result['summary'] = apply_plan(job['plan'], state)
        elif phase == 'auto':
            result['summary'] = automatic_mesh(job, state)
        elif phase == 'generate':
            verify_live_boundaries(state)
            if int(Model.Mesh.Elements) > 0:
                raise RuntimeError('Existing mesh found; automatic remeshing is disabled by default')
            start = time.time()
            Model.Mesh.GenerateMesh()
            result['summary'] = {'seconds': time.time() - start, 'elements': int(Model.Mesh.Elements),
                                 'nodes': int(Model.Mesh.Nodes)}
            if int(Model.Mesh.Elements) <= 0:
                raise RuntimeError('GenerateMesh returned without valid elements')
        elif phase == 'quality_check':
            # Explicit read-only reporting. Do not run meshing, modify control
            # scopes, create named selections, or export any mesh.
            emit_progress(68,'读取真实网格质量','正在从当前 Meshing MeshMetric 读取可用指标')
            count=int(Model.Mesh.Elements)
            if count<=0:
                result['summary']={'mesh_elements':0,'quality_status':'not_measured',
                    'quality_gate':'no_mesh','next_action':'当前工程没有可用网格；无法测量网格质量'}
            else:
                quality=assess_quality()
                verdict=reviewed_quality_gate(quality)
                result['summary']={'mesh_elements':count,'quality':quality,
                    'quality_gate':verdict['quality_gate'],
                    'quality_status':verdict['status'],
                    'next_action':verdict['reason'],
                    'worksheet_diagnostics':quality_worksheet_summary(),
                    'quality_source':'ACT 当前 Meshing 实测（未执行 GenerateMesh）',
                    'geometry_fingerprint':state['fingerprint'], 'project':state['project'],
                    'mesh_reference':mesh_reference_status(state)}
        elif phase == 'quality':
            result['summary'] = assess_quality()
        elif phase == 'mark_failed':
            result['summary']=mark_official_quality_elements('failed')
        elif phase == 'mark_warning':
            result['summary']=mark_official_quality_elements('warning')
            warning_register_marker(result['summary'],state)
        elif phase == 'mark_high_skew':
            emit_progress(69,'定位指定偏度范围','在当前网格 Named Selection Worksheet 按双端 Skewness 条件筛选，原质量阈值保持不变')
            cancellation_checkpoint()
            # Retire the active marker first. Old selection objects may remain
            # in Meshing for the user's history, but an unsuccessful new scan
            # must never silently reuse the former range.
            write(_warning_path(_WARNING_MARKER), {'status':'pending',
                'range_min':job.get('skew_min',0.90),'range_max':job.get('skew_max',1.00),
                'project':state['project'],'fingerprint':state['fingerprint'],
                'groups':[]})
            result['summary']=mark_threshold_skew_elements(job.get('skew_min',0.90),job.get('skew_max',1.00))
            warning_register_marker(result['summary'],state)
        elif phase == 'warning_refine_preview':
            emit_progress(70,'定位警告单元','从刚标记的官方 Worksheet 组提取坐标和对应几何体')
            cancellation_checkpoint()
            result['summary']=preview_warning_refinement(state)
        elif phase == 'warning_refine_publish':
            emit_progress(70,'发布局部球形 Sizing','只为已审核警告区创建可编辑的球形控制，不重划网格')
            cancellation_checkpoint()
            result['summary']=publish_warning_refinement(state)
        elif phase == 'diagnose':
            count=int(Model.Mesh.Elements)
            if count<=0:
                result['summary']={'mesh_elements':0,
                  'worksheet_diagnostics':{'status':'no_mesh','metrics':[]},
                  'element_probe':{'status':'no_mesh','checked_element_count':0,
                                   'sampling_fraction':0,'samples':[]},
                  'next_action':'No generated mesh exists; generate or load mesh first'}
            else:
                emit_progress(66,'诊断网格','读取官方质量统计；准备定位采样网格元素')
                results={'mesh_elements':count, 'quality':assess_quality(),
                         'quality_gate':None,
                         'worksheet_diagnostics':quality_worksheet_summary(),
                         'mesh_reference':mesh_reference_status(state),
                         'geometry_fingerprint':state['fingerprint'],
                         'project':state['project']}
                verdict=reviewed_quality_gate(results['quality'])
                results['quality_gate']=verdict['quality_gate']
                results['quality_status']=verdict['status']
                results['next_action']=verdict['reason']
                emit_progress(70,'采样定位','正在检查四面体形状和单元坐标；这是采样诊断，不是全网格检索')
                results['element_probe']=probe_tet_geometry()
                result['summary']=results

        elif phase == 'export':
            result['summary'] = guarded_export(job,state)
        else:
            raise ValueError('Unsupported phase')
    result['status'] = 'ok'
    emit_progress(100,'任务完成','操作结束，结果已写入报告')
except AutoMeshCancelled as e:
    result['status'] = 'cancelled'
    result['error'] = utext(e)
    emit_progress(None,'已安全取消','ACT 在安全检查点确认取消，未强制结束 Meshing 进程')
except Exception:
    result['status'] = 'error'
    result['error'] = traceback.format_exc()
    emit_progress(None,'任务失败',utext(result['error']).splitlines()[-1][:180])
# Persist the job-specific terminal receipt first. A user may have an old
# quality_check in last_result.json; the UI must not confuse it with this job.
# Independent writes: losing one output must not discard the other. The Go
# supervisor can recover a terminal failure from this job's final ACT marker.
_final_write_errors = []
_final_write_ok = []
for _final_path in (os.path.join(os.path.dirname(RESULT_FILE), "job_%s.json" % jobid), RESULT_FILE):
    try:
        write(_final_path, result)
        _final_write_ok.append(_final_path)
    except Exception as _final_exc:
        _final_write_errors.append('%s: %s' % (_final_path, utext(_final_exc)))
if _final_write_errors:
    # Keep a human-readable, job-specific fallback diagnosis, never guess
    # whether Sizing was committed from a missing JSON receipt.
    details='; '.join(_final_write_errors)
    try:
        with open(os.path.join(os.path.dirname(RESULT_FILE),
                  'job_receipt_write_error_%s.txt' % jobid), 'wb') as _err_log:
            _err_log.write(details.encode('utf-8','replace'))
    except Exception:
        pass
    emit_progress(None, ('部分回执写入失败' if _final_write_ok else '终态结果全部写入失败'), details[:300])
#                                                    
#                                       

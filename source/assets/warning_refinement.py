# -*- coding: utf-8 -*-
"""R5.2: conservative quality-warning hotspot assistance for Mechanical ACT.

The official quality worksheet creates *element* selections. A persistent Sizing
is instead attached to a geometry body and constrained by sphere of influence.
This module never regenerates/clears mesh; every write requires explicit review.
IronPython 2.7 / Python 3 compatible. Injected by meshing_worker.py.
"""
import os
import math
import hashlib
import json

_WARNING_MARKER = 'warning_refinement_marker.json'
_WARNING_PLAN = 'warning_refinement_plan.json'
_WARNING_MANAGED = 'warning_refinement_managed.json'
_WARNING_SELECTION = 'warning_refinement_selection.json'
# 64 is a per-influence-domain cap, not a global rejection threshold.
MAX_WARNINGS = 64
# Full ACT element / node traversal is bounded independently for 32 GB hosts.
MAX_ANALYSIS_ELEMENTS = 4096
# At most 6 active changes are processed per transaction batch. The whole
# reviewed plan can contain several bounded batches (one user action).
# Cumulative cell proxy is advisory; geometry and per-batch guards remain.
MAX_BATCH_SPHERES = 6
MAX_SPHERES = 4096  # analysis record hard cap, NOT a publish limit
MAX_RADIUS_MM = 12.0
MAX_CELL_PROXY = 120000.0  # advisory cumulative proxy; each giant single sphere remains invalid


def _warning_path(filename):
    return os.path.join(os.path.dirname(JOB_FILE), filename)


def _live_warning_qr_controls():
    return [x for x in Model.Mesh.Children if utext(getattr(x, 'Name', '')).startswith('AutoMesh_QR_')]


def _live_warning_qr_coordinate_systems():
    root = getattr(Model, 'CoordinateSystems', None)
    if root is None:
        return []
    return [x for x in root.Children if utext(getattr(x, 'Name', '')).startswith('AutoMesh_QR_CS_')]


def _adopt_live_warning_controls(state):
    """Create a read-back ledger for reserved QR controls in the live ACT model.

    This path is used only when the shared manifest is missing or belongs to a
    different project/geometry. It does not trust old candidate IDs. It reads
    each live Sizing and its paired coordinate system directly, verifies the
    exact Body scope, type, units, center, radius and size, and never edits the
    existing objects. The adopted ledger is persisted only after a successful
    new publication batch.
    """
    live = _live_warning_qr_controls()
    coordinate_systems = _live_warning_qr_coordinate_systems()
    if not live:
        return {'schema_version': 3, 'project': state['project'],
                'fingerprint': state['fingerprint'], 'controls': [], 'rounds': [],
                'adopted_from_live_model': True}

    valid_bodies = set(int(x['id']) for x in state.get('bodies', []))
    cs_by_key = {}
    for cs in coordinate_systems:
        key = (utext(cs.Name), int(cs.ObjectId))
        if key in cs_by_key:
            raise RuntimeError(u'发现重复的 AutoMesh 球心坐标系名称/对象 ID，不能安全接管。')
        cs_by_key[key] = cs

    entries = []
    used_cs = set()
    used_names = set()
    used_ids = set()
    for control in live:
        name = utext(control.Name)
        object_id = int(control.ObjectId)
        if name in used_names or object_id in used_ids:
            raise RuntimeError(u'发现重复的 AutoMesh_QR Sizing 身份，不能安全接管。')
        used_names.add(name); used_ids.add(object_id)
        if not _is_sizing(control):
            raise RuntimeError(u'保留名称 AutoMesh_QR_ 对应对象并非可识别 Sizing：' + name)
        if utext(getattr(control, 'Type', '')) != utext(SizingType.SphereOfInfluence):
            raise RuntimeError(u'历史 AutoMesh_QR Sizing 不是 Sphere of Influence，需人工核对：' + name)
        if _is_control_suppressed(control):
            raise RuntimeError(u'历史 AutoMesh_QR Sizing 已被停用；请先人工核对启用状态：' + name)
        scope = _effective_sizing_ids(control)
        if len(scope) != 1:
            raise RuntimeError(u'历史 AutoMesh_QR Sizing 不是唯一 Body 作用域，不能安全接管：' + name)
        body_id = list(scope)[0]
        if body_id not in valid_bodies:
            raise RuntimeError(u'历史 AutoMesh_QR Sizing 的 Body 不属于当前几何：' + name)
        try:
            cs_obj = control.SphereCenter
            cs_key = (utext(cs_obj.Name), int(cs_obj.ObjectId))
        except Exception:
            raise RuntimeError(u'历史 AutoMesh_QR Sizing 缺少可读取的球心坐标系：' + name)
        if not cs_key[0].startswith('AutoMesh_QR_CS_') or cs_key not in cs_by_key:
            raise RuntimeError(u'历史 AutoMesh_QR Sizing 的球心坐标系无法唯一匹配：' + name)
        if cs_key in used_cs:
            raise RuntimeError(u'多个历史 Sizing 共用同一球心坐标系，不能自动接管：' + name)
        used_cs.add(cs_key)
        suffix = name[len('AutoMesh_QR_'):]
        if cs_key[0] != 'AutoMesh_QR_CS_' + suffix:
            raise RuntimeError(u'历史 Sizing 与球心坐标系命名配对不一致：' + name)
        size = _read_control_mm(control)
        radius = _length_mm(control.SphereRadius)
        centre = _origin_coords_mm(cs_by_key[cs_key])
        if not (0.5 <= size <= 5.0 and 0.0 < radius <= MAX_RADIUS_MM):
            raise RuntimeError(u'历史 Sizing 尺寸或半径超出可核验范围：' + name)
        if len(centre) != 3 or any(math.isnan(float(v)) or math.isinf(float(v)) for v in centre):
            raise RuntimeError(u'历史 Sizing 球心坐标无效：' + name)
        proxy = int(math.ceil((4.0 / 3.0) * math.pi * (radius / size) ** 3))
        entries.append({'candidate_id': '', 'name': name, 'object_id': object_id,
                        'coordinate_system': cs_key[0], 'cs_object_id': cs_key[1],
                        'body_id': body_id, 'radius_mm': radius, 'size_mm': size,
                        'centre_mm': [float(v) for v in centre],
                        'cell_budget_proxy': proxy, 'adopted_from_live_model': True})

    if len(used_cs) != len(coordinate_systems):
        orphaned = [utext(x.Name) for x in coordinate_systems
                    if (utext(x.Name), int(x.ObjectId)) not in used_cs]
        raise RuntimeError(u'发现未配对的 AutoMesh_QR 球心坐标系：' + ', '.join(orphaned[:8]))
    return {'schema_version': 3, 'project': state['project'],
            'fingerprint': state['fingerprint'], 'controls': entries, 'rounds': [],
            'adopted_from_live_model': True}


def _current_warning_owner(state):
    """Load the current ledger or safely adopt verified live QR controls.

    Mesh element IDs and candidate IDs are mesh-round-specific. Existing ACT
    Sizing objects are persistent controls and are validated by their live
    object identities and properties, not by requiring old candidate IDs to
    appear in a newly remeshed candidate list.
    """
    filename = _warning_path(_WARNING_MANAGED)
    owner = {}
    manifest_error = None
    if os.path.isfile(filename):
        try:
            owner = read(filename)
        except Exception as exc:
            manifest_error = exc
    live = _live_warning_qr_controls()
    if manifest_error is not None:
        if live:
            return _adopt_live_warning_controls(state)
        # A malformed shared manifest with no live QR Sizing in this model is
        # stale metadata, not a reason to block a fresh model. Orphan QR
        # coordinate systems are still checked before any publication.
        return {}
    same_model = bool(owner) and owner.get('project') == state['project'] and owner.get('fingerprint') == state['fingerprint']
    if same_model:
        if owner.get('controls') or not live:
            return owner
        return _adopt_live_warning_controls(state)
    if live:
        return _adopt_live_warning_controls(state)
    return {}

def warning_register_marker(summary, state):
    """Persist the identity of the newly created quality-element selection group."""
    groups = summary.get('created_groups') or []
    root = Model.NamedSelections
    tagged = []
    for name in groups:
        candidates = [s for s in root.Children if utext(s.Name) == utext(name)]
        if len(candidates) != 1:
            continue
        s = candidates[0]
        try:
            tagged.append({'name': utext(name), 'object_id': int(s.ObjectId),
                           'ids': sorted(int(v) for v in s.Location.Ids)})
        except Exception:
            pass
    info = {'status': 'valid' if summary.get('status') == 'created' and tagged else 'unavailable',
            'project': state['project'], 'fingerprint': state['fingerprint'],
            'mesh_count': int(Model.Mesh.Elements), 'groups': tagged,
            'worksheet_count': summary.get('worksheet_count', 0),
            'selection_source':summary.get('selection_source','official_quality_warning'),
            'metric':summary.get('metric','Skewness'),
            'skewness_threshold':summary.get('threshold'),
            'range_min':summary.get('range_min'),'range_max':summary.get('range_max'),
            'requested_range_max':summary.get('requested_range_max'),
            'auto_limited':bool(summary.get('auto_limited'))}
    write(_warning_path(_WARNING_MARKER), info)
    return info


def _read_warning_elements(state):
    filename = _warning_path(_WARNING_MARKER)
    if not os.path.isfile(filename):
        raise RuntimeError(u'请先按偏度范围标记当前网格的 Mesh Elements。')
    marker = read(filename)
    if marker.get('status') != 'valid' or not marker.get('groups'):
        raise RuntimeError(u'未找到当前网格的有效偏度标记组；请重新选择偏度范围并标记。')
    if marker.get('project') != state['project'] or marker.get('fingerprint') != state['fingerprint']:
        raise RuntimeError(u'工程或几何变化，旧警告组不可复用。请重新检查质量并标记警告。')
    if int(marker.get('mesh_count', -1)) != int(Model.Mesh.Elements):
        raise RuntimeError(u'网格单元数量已变化；警告单元 ID 不再可信，请重新标记。')
    selected = []
    names = {}
    for ns in Model.NamedSelections.Children:
        names.setdefault(utext(ns.Name), []).append(ns)
    for entry in marker['groups']:
        candidates = names.get(entry['name'], [])
        if len(candidates) != 1 or int(candidates[0].ObjectId) != int(entry['object_id']):
            raise RuntimeError(u'警告命名组被修改、重命名或删除；请重新标记。')
        live = sorted(int(v) for v in candidates[0].Location.Ids)
        if live != entry['ids']:
            raise RuntimeError(u'警告单元组作用域已经变化；拒绝使用旧坐标。')
        selected.extend(live)
    selected = sorted(set(selected))
    if not selected:
        raise RuntimeError(u'当前偏度选择组为空；请重新标记。')
    if len(selected) != int(marker['worksheet_count']):
        raise RuntimeError(u'标记数量与实际选择组单元总数不一致，请重新执行偏度范围标记。')
    if len(selected) > MAX_ANALYSIS_ELEMENTS:
        # This is a separate upper bound on ACT node reads, not an all-or-none
        # 64-element hotspot limit. Never allocate or scan arbitrary millions.
        return marker, [], 'analysis_limit:%d' % len(selected)
    data = ExtAPI.DataModel.MeshDataByName('Global')
    factor = unit_to_mm(utext(data.Unit))
    if factor is None:
        raise RuntimeError(u'网格坐标单位无法确认（MeshData.Unit）；拒绝自动设置球心，以防 mm/m 错误。')
    allowed_body_ids = set(int(b['id']) for b in state['bodies'])
    face_to_body = dict((int(f['id']),int(f['body'])) for f in state.get('faces',[]))
    observations = []
    rejected = []
    for selected_index, eid in enumerate(selected):
        cancellation_checkpoint()
        if selected_index == 0 or (selected_index + 1) % 16 == 0 or selected_index + 1 == len(selected):
            try:
                pct = 72 + int(5.0 * (selected_index + 1) / max(1, len(selected)))
                emit_progress(pct, '提取标记单元几何', '%d/%d 个单元' % (selected_index + 1, len(selected)))
            except Exception:
                pass
        try:
            element = data.ElementById(int(eid))
            node_ids = sorted(set(int(v) for v in element.CornerNodeIds))
            if len(node_ids) < 4:
                raise ValueError('too_few_corner_nodes')
            nodes = [data.NodeById(i) for i in node_ids]
            pts = [(float(nd.X)*factor, float(nd.Y)*factor, float(nd.Z)*factor) for nd in nodes]
            if any(math.isnan(v) or math.isinf(v) for point in pts for v in point):
                raise ValueError('non_finite_coordinates')
            centroid = [sum(p[i] for p in pts)/len(pts) for i in range(3)]
            span = max(_distance(pts[i],pts[j]) for i in range(len(pts)) for j in range(i+1,len(pts)))
            if span <= 0 or span > 500:
                raise ValueError('invalid_element_span')
            # Trust direct ACT geometry ownership where available. Fallback to
            # intersection of corner-node BodyIds; no global guess across bodies.
            direct = set()
            for prop in ('BodyId','GeoBodyId','GeometryBodyId'):
                try:
                    value = getattr(element,prop)
                    if int(value) in allowed_body_ids:
                        direct.add(int(value))
                except Exception:
                    pass
            mapped_faces = set()
            for prop in ('GeoEntityId','GeometryEntityId','FaceId'):
                try:
                    val = int(getattr(element,prop))
                    if val in face_to_body:
                        direct.add(face_to_body[val])
                        mapped_faces.add(val)
                except Exception:
                    pass
            by_node = []
            for nd in nodes:
                try:
                    ids = set(int(v) for v in nd.BodyIds).intersection(allowed_body_ids)
                    if ids: by_node.append(ids)
                except Exception:
                    pass
            common = set.intersection(*by_node) if by_node else set()
            mapped = direct or common
            method = ('element_geometry' if direct else 'node_body_intersection')
            if len(mapped) != 1:
                if not by_node and not direct and len(allowed_body_ids) == 1:
                    mapped = set(allowed_body_ids)
                    method = 'unique_body_fallback_requires_review'
                else:
                    raise ValueError('ambiguous_geometry_body')
            body_id = list(mapped)[0]
            if by_node and any(body_id not in ids for ids in by_node):
                raise ValueError('inconsistent_node_body_ownership')
            skew=None
            # ACT IElement does NOT guarantee per-element Skewness in 2024R2.
            # Only use an actual numeric exposed metric. Never interpolate
            # from the worksheet range or estimate from corner-node geometry.
            try:
                value=getattr(element,'Skewness')
                if not callable(value) and not isinstance(value,bool):
                    v=float(value)
                    if not math.isnan(v) and not math.isinf(v) and 0<=v<=1:
                        skew=v
            except Exception:
                pass
            # An ElementQuality worksheet identifies the affected element IDs,
            # but does not imply exact per-element values. Read only a real
            # ACT property when exposed; never infer quality from the interval.
            element_quality = None
            if marker.get('metric') == 'ElementQuality':
                try:
                    value = getattr(element, 'ElementQuality')
                    if not callable(value) and not isinstance(value, bool):
                        v = float(value)
                        if not math.isnan(v) and not math.isinf(v) and 0 <= v <= 1:
                            element_quality = v
                except Exception:
                    pass
            observations.append({'id':eid,'body':body_id,'xyz_mm':centroid,
                                 'skewness':skew, 'element_quality':element_quality,
                                 'span_mm':span,'node_ids':node_ids,
                                 'mapped_face_ids':sorted(mapped_faces),
                                 'mapping_method':method})
        except (KeyboardInterrupt, SystemExit):
            raise
        except Exception as exc:
            # One bad/multi-body element must not discard other safe clusters.
            reason = utext(exc)
            if not isinstance(exc, ValueError):
                reason = 'element_geometry_unreadable'
            rejected.append({'element_ids':[eid],'reason':reason})
    marker['_element_rejections'] = rejected
    digest_payload = ['%s:%s:%s:%s:%s' % (o['id'],o['body'],
        ','.join('%.9g' % n for n in o['xyz_mm']),
        ','.join(str(v) for v in o['node_ids']),
        ('%.12g' % o['skewness'] if o.get('skewness') is not None else 'unknown') + ':' +
        ('%.12g' % o['element_quality'] if o.get('element_quality') is not None else 'unknown') + ':' +
        ','.join(str(v) for v in o.get('mapped_face_ids', [])))
        for o in observations]
    digest_payload.extend('rejected:%s:%s' % (r['element_ids'][0],r['reason']) for r in rejected)
    digest = hashlib.sha256(('|'.join(digest_payload)).encode('utf-8')).hexdigest()
    return marker, observations, digest


def _distance(a,b):
    return math.sqrt(sum((a[i]-b[i])**2 for i in range(3)))


def _cluster_radius(items):
    centre = [sum(x['xyz_mm'][i] for x in items)/float(len(items)) for i in range(3)]
    # Covers each member's actual centroid plus a conservative local span.
    radius = max(1.5,max(_distance(centre,x['xyz_mm']) + 0.75*x['span_mm'] for x in items))
    return centre,radius


def _connected_hotspot_components(observations):
    """Bounded grid-hash neighbor search instead of O(n²) global comparisons.

    The node adjacency path still joins two elements sharing >=2 corners on
    the SAME geometry body. Proximity checks inspect only 27 local 3 mm cells.
    """
    values=sorted(observations,key=lambda x:(x['body'],x['id']))
    parent=list(range(len(values)))
    def root(i):
        while parent[i]!=i:
            parent[i]=parent[parent[i]]
            i=parent[i]
        return i
    def link(i,j):
        a,b=root(i),root(j)
        if a!=b: parent[max(a,b)]=min(a,b)
    grid={}; node_map={}
    for i,item in enumerate(values):
        cancellation_checkpoint()
        body=item['body']
        # Share two or more corner nodes, not merely a single wafer tip.
        neigh={}
        for node in item.get('node_ids',[]):
            for j in node_map.get((body,node),[]):
                neigh[j]=neigh.get(j,0)+1
        for j,count in neigh.items():
            if count>=2: link(i,j)
        centre=item['xyz_mm']
        key=tuple(int(math.floor(v/3.0)) for v in centre)
        for dx in (-1,0,1):
            for dy in (-1,0,1):
                for dz in (-1,0,1):
                    for j in grid.get((body,key[0]+dx,key[1]+dy,key[2]+dz),[]):
                        prev=values[j]
                        proximity=min(3.0,1.4*min(item['span_mm'],prev['span_mm']))
                        if _distance(centre,prev['xyz_mm'])<=proximity:link(i,j)
        grid.setdefault((body,)+key,[]).append(i)
        for node in item.get('node_ids',[]):
            node_map.setdefault((body,node),[]).append(i)
    groups={}
    for i,value in enumerate(values):groups.setdefault(root(i),[]).append(value)
    return sorted(groups.values(),key=lambda g:(-len(g),g[0]['body'],min(x['id'] for x in g)))


def _split_component(component):
    """Partition a connected region into bounded spheres, never one huge ball.

    A long arc or several separated fine-feature regions may require several
    domains. Each domain has at most 64 marked members; the 12 mm radius cap is
    enforced during growth, not after accepting a whole region.
    """
    remaining=list(component)
    partitions=[]
    while remaining:
        cancellation_checkpoint()
        seed=min(remaining,key=lambda x:x['id'])
        ordered=sorted(remaining,key=lambda x:(_distance(x['xyz_mm'],seed['xyz_mm']),x['id']))
        group=[seed]
        for item in ordered:
            if item is seed: continue
            if len(group)>=MAX_WARNINGS:break
            _,radius=_cluster_radius(group+[item])
            if radius<=MAX_RADIUS_MM:
                group.append(item)
        ids=set(x['id'] for x in group)
        remaining=[x for x in remaining if x['id'] not in ids]
        partitions.append(group)
    return partitions


def _hotspot_candidates(observations, body_size_caps=None, diagnosis=None, fingerprint='', metric='Skewness'):
    """Enumerate ALL individually safe candidates; do not spend the publish
    budget during analysis, since the operator decides which ones to select.
    """
    body_size_caps=body_size_caps or {}
    raw=[]
    for component in _connected_hotspot_components(observations):
        raw.extend(_split_component(component))
    if len(raw)>MAX_SPHERES:
        raise RuntimeError(u'候选区域数超过 %d 条一次性安全处理上限；没有截断，请按 Body 或范围分批分析。' % MAX_SPHERES)
    output=[]
    for cl in raw:
        cancellation_checkpoint()
        ids=sorted(x['id'] for x in cl)
        body=cl[0]['body']
        centre,radius=_cluster_radius(cl)
        max_span=max(x['span_mm'] for x in cl)
        if radius>MAX_RADIUS_MM:
            if diagnosis is not None:diagnosis.append({'element_ids':ids,'reason':'influence_radius_exceeds_limit','proposed_radius_mm':round(radius,4)})
            continue
        target=max(0.5,min(5.0,0.75*max_span))
        if body in body_size_caps:
            target=min(target,0.9*body_size_caps[body])
            if target<0.5:
                if diagnosis is not None:diagnosis.append({'element_ids':ids,'reason':'already_has_finer_control'})
                continue
        proxy=(4.0/3.0)*math.pi*(radius/target)**3
        if proxy>MAX_CELL_PROXY:
            if diagnosis is not None:diagnosis.append({'element_ids':ids,'reason':'single_candidate_budget_exceeded', 'cell_budget_proxy':int(round(proxy))})
            continue
        known=[float(x['skewness']) for x in cl if x.get('skewness') is not None]
        complete=len(known)==len(cl)
        eq_known=[float(x['element_quality']) for x in cl if x.get('element_quality') is not None]
        eq_complete=len(eq_known)==len(cl)
        # Candidate identity cannot depend on list position or sort order.
        id_basis='%s|%s|%s' % (fingerprint,body,','.join(str(n) for n in ids))
        candidate_id=hashlib.sha256(id_basis.encode('utf-8')).hexdigest()[:20]
        observed_faces = sorted(set(face_id for item in cl for face_id in item.get('mapped_face_ids', [])))
        output.append({'candidate_id':candidate_id,'body_id':body,
                       'mapped_face_ids':observed_faces[:12],
                       'centre_mm':[round(x,6) for x in centre],
                       'radius_mm':round(radius,4),'element_size_mm':round(target,4),
                       'element_ids':ids,'observed_max_span_mm':round(max_span,4),
                       'cell_budget_proxy':int(round(proxy)),
                       'skewness_max':max(known) if complete else None,
                       'skewness_mean':(sum(known)/len(known)) if complete else None,
                       'skewness_verified':complete,
                       'skewness_source':('ACT:IElement.Skewness' if complete else 'unavailable'),
                       'element_quality_min':min(eq_known) if eq_complete else None,
                       'element_quality_mean':(sum(eq_known)/len(eq_known)) if eq_complete else None,
                       'element_quality_verified':eq_complete,
                       'element_quality_source':('ACT:IElement.ElementQuality' if eq_complete else 'unavailable'),
                       'scope_kind':'GeometryBody',
                       'mapping_methods':sorted(set(x.get('mapping_method','unknown') for x in cl)),
                       'wafer_gap_clearance':'manual_review_required'})
    # Verified actual maxima first, otherwise explicit "unavailable", then
    # stable diagnostic priority. Never manufacture an exact ranking from IDs.
    if metric == 'ElementQuality':
        # Lowest *verified* element quality first. Unknown values remain
        # unknown and are ordered by defect count only, never by fake quality.
        output.sort(key=lambda c:(c['element_quality_min'] is None,
                         c['element_quality_min'] if c['element_quality_min'] is not None else 0,
                         -len(c['element_ids']),c['body_id'],c['candidate_id']))
    else:
        output.sort(key=lambda c:(c['skewness_max'] is None,
                          -(c['skewness_max'] if c['skewness_max'] is not None else 0),
                          -len(c['element_ids']),c['body_id'],c['candidate_id']))
    for i,c in enumerate(output):c['batch_index']=i//MAX_BATCH_SPHERES+1
    return output,sum(c['cell_budget_proxy'] for c in output)


def _existing_body_size_caps(state):
    """Account for all verifiable active local controls; never overwrite manual.

    Current AutoMesh hotspot controls must be verified by the durable ownership
    manifest, then excluded from the *replacement* size comparison. Counting an
    owned SOI as a new manual cap would shrink the mesh again on every preview.
    """
    valid_bodies=set(int(b['id']) for b in state['bodies'])
    face_to_body=dict((int(f['id']),int(f['body'])) for f in state['faces'])
    caps={}
    owner=_current_warning_owner(state)
    owned=set((e.get('name'),int(e.get('object_id',-1))) for e in owner.get('controls',[]))
    unresolved=[]
    for control in Model.Mesh.Children:
        if not _is_sizing(control) or _is_control_suppressed(control):
            continue
        if utext(control.Name).startswith('AutoMesh_QR_'):
            if (utext(control.Name),int(control.ObjectId)) not in owned:
                raise RuntimeError(u'发现未确认归属的警告区 Sizing；禁止覆盖或叠加。')
            continue
        mm=_read_control_mm(control)
        if mm<=0 or math.isnan(mm) or math.isinf(mm):
            raise RuntimeError(u'存在无法确认的局部尺寸。')
        ids=_effective_sizing_ids(control)
        affected=set()
        unknown=set()
        for ident in ids:
            if ident in valid_bodies:affected.add(ident)
            elif ident in face_to_body:affected.add(face_to_body[ident])
            else:unknown.add(ident)
        if unknown:
            unresolved.append((mm,unknown))
        if not affected and not unknown:
            raise RuntimeError(u'已有 Sizing 缺少有效作用域。')
        for body in affected:
            caps[body]=min(caps.get(body,mm),mm)
    if unresolved:
        # Resolve ACT Edge IDs to their owning Body only when necessary.
        edge_map={}
        wanted=set(x for _, ids in unresolved for x in ids)
        try:
            for assembly in ExtAPI.DataModel.GeoData.Assemblies:
                for part in assembly.Parts:
                    for body in part.Bodies:
                        bid=int(body.Id)
                        for face in body.Faces:
                            for edge in face.Edges:
                                eid=int(edge.Id)
                                if eid in wanted:
                                    edge_map.setdefault(eid,set()).add(bid)
        except Exception:
            raise RuntimeError(u'现有 Edge Sizing 作用域无法核查；拒绝发布可能覆盖手工控制的建议。')
        for mm,ids in unresolved:
            for eid in ids:
                scope=edge_map.get(eid,set())
                if len(scope)!=1:
                    raise RuntimeError(u'现有 Sizing 的 Face/Edge 作用域归属不唯一；请人工核对。')
                bid=list(scope)[0]
                caps[bid]=min(caps.get(bid,mm),mm)
    return caps


def preview_warning_refinement(state):
    cancellation_checkpoint()
    emit_progress(71, '读取偏度标记组', '核验标记组、网格数量和当前几何身份')
    marker, observations, digest=_read_warning_elements(state)
    if not observations and digest.startswith('analysis_limit:'):
        amount=int(digest.split(':')[1])
        proposal={'schema_version':1,'status':'no_safe_candidates','project':state['project'],
                 'geometry_fingerprint':state['fingerprint'],'mesh_elements':int(Model.Mesh.Elements),
                 'warning_count':amount,'covered_count':0,'uncovered_count':amount,
                 'range_min':marker.get('range_min'),'range_max':marker.get('range_max'),
                 'job_id':utext(job.get('job_id','')),
                 'group_count':0,'clusters':[],'total_cell_proxy':0,
                 'notes':u'选中单元超过 ACT 一次性安全读取上限；建议缩小范围或将范围分段分批筛选（未修改原网格）。',
                 'rejected_groups':[{'element_ids':[], 'count':amount,'reason':'analysis_element_count_limit', 'limit':MAX_ANALYSIS_ELEMENTS}]}
        proposal['plan_id']=hashlib.sha256((utext(state['fingerprint'])+':'+digest).encode('utf-8')).hexdigest()[:20]
        write(_warning_path(_WARNING_PLAN),proposal)
        return proposal
    emit_progress(79, '核对已有 Sizing', '检查现有局部尺寸，避免覆盖更细的手工控制')
    caps=_existing_body_size_caps(state)
    emit_progress(86, '空间分簇与预算审核', '按几何 Body、邻接关系和影响半径生成独立候选')
    rejected=list(marker.get('_element_rejections',[]))
    groups,proxy=_hotspot_candidates(observations,caps,rejected,state['fingerprint'],marker.get('metric','Skewness'))
    # No safe candidate is NOT a failed task. Record why, without creating Sizing.
    # The caller can open this report while the Meshing mesh stays untouched.
    covered=set(i for x in groups for i in x['element_ids'])
    proposal={'schema_version':1,'status':('ready' if groups else 'no_safe_candidates'),'project':state['project'],
              'geometry_fingerprint':state['fingerprint'],
              'job_id':utext(job.get('job_id','')),
              'mesh_elements':int(Model.Mesh.Elements),'warning_digest':digest,
              'warning_count':len(observations)+len(marker.get('_element_rejections',[])),
              'range_min':marker.get('range_min'),'range_max':marker.get('range_max'),'group_count':len(groups),'batch_count':(len(groups)+MAX_BATCH_SPHERES-1)//MAX_BATCH_SPHERES,
              'covered_count':len(covered),'uncovered_count':len(observations)+len(marker.get('_element_rejections',[]))-len(covered),
              'clusters':groups,'rejected_groups':rejected,'total_cell_proxy':int(round(proxy)),
              'candidate_count':len(groups),'candidate_max_skew_verified':sum(1 for g in groups if g['skewness_verified']),
              'source':('Named Selection Worksheet: Mesh Element ElementQuality %.3g–%.3g' % (marker.get('range_min'), marker.get('range_max')) if marker.get('selection_source')=='named_selection_worksheet_element_quality_range' else ('Named Selection Worksheet: Mesh Element Skewness %.3g–%.3g' % (marker.get('range_min'), marker.get('range_max')) if marker.get('selection_source')=='named_selection_worksheet_skewness_range' else 'ANSYS Mesh Quality Worksheet element Named Selection')),
              'quality_metric':marker.get('metric','Skewness'),
              'skewness_threshold':marker.get('skewness_threshold'),
              'notes':u'完整列出所有可分析局部候选（不按 24 组截断）。发布时才审核选中集合的累计预算，每批最多 6 个；真实偏度不可用则明确显示未知；晶圆间净空仍需逐候选人工确认。'
             }
    emit_progress(94, '保存局部方案', '候选 %d 组；正在写入本任务专属方案文件' % len(groups))
    proposal['plan_id']=hashlib.sha256(json.dumps(
        [proposal['project'],proposal['geometry_fingerprint'],proposal['warning_digest'],groups],
        sort_keys=True,ensure_ascii=True).encode('utf-8')).hexdigest()[:16]
    write(_warning_path(_WARNING_PLAN),proposal)
    return dict(proposal)


def _length_mm(value):
    """Read only an explicitly unit-qualified ACT length; never guess units."""
    import re
    st=utext(value).strip()
    mt=re.search(r'([-+]?\d+(?:\.\d+)?(?:[Ee][-+]?\d+)?)\s*\[?\s*(mm|cm|m|um|in|inch)\b',st,re.I)
    if mt is None:
        raise RuntimeError(u'旧局部控制的长度单位不可确认：'+st[:70])
    factor=unit_to_mm(mt.group(2))
    if factor is None: raise RuntimeError(u'未知尺寸单位')
    return float(mt.group(1))*factor


def _origin_coords_mm(cs):
    if all(hasattr(cs, k) for k in ('OriginX','OriginY','OriginZ')):
        return [_length_mm(getattr(cs,k)) for k in ('OriginX','OriginY','OriginZ')]
    # Mock/older API location is used only for ownership readback; actual ACT
    # updates must verify that one of the supported origin APIs is available.
    if hasattr(cs,'origin'):
        return [_length_mm(v) for v in cs.origin]
    raise RuntimeError(u'无法读取已发布球心，不能确定旧坐标系是否被人工调整。')


def _set_origin_mm(cs, xyz):
    components=[Quantity('%s [mm]' % v) for v in xyz]
    # 2024 R2 SetOriginLocation is the documented operation. R7.5 incorrectly
    # assigned CoordinateSystemAlignmentType.Free to OriginDefineBy: that enum
    # belongs to AXIS alignment, not coordinate-system ORIGIN definition.
    if hasattr(cs, 'SetOriginLocation'):
        cs.SetOriginLocation(*components)
    elif all(hasattr(cs,k) for k in ('OriginX','OriginY','OriginZ')):
        # Do not set OriginDefineBy to an unrelated enum. For versions exposing
        # only properties, require a verifiable compatible origin mode.
        if hasattr(cs,'OriginDefineBy'):
            current = cs.OriginDefineBy
            descriptor = utext(current).lower()
            if not ('global' in descriptor or 'coordinate' in descriptor):
                raise RuntimeError(u'球心坐标系未提供 SetOriginLocation，且原点定义方式不是可验证的全局坐标；拒绝猜测 ACT 枚举。')
        cs.OriginX=components[0]
        cs.OriginY=components[1]
        cs.OriginZ=components[2]
    else:
        raise RuntimeError(u'ACT 未提供可核验的球心属性')


def _owned_live_controls(owned, existing):
    """Resolve exact Name+ObjectId ownership and verify all live properties.

    Existing controls are never resized or moved by incremental publication;
    they are only validated and included in the cumulative resource budget.
    """
    recorded=owned.get('controls',[])
    if not recorded:
        if existing:
            raise RuntimeError(u'当前存在 AutoMesh_QR Sizing，但没有完整接管记录；拒绝叠加发布。')
        return []
    live_list=list(existing)
    live=dict(((utext(x.Name),int(x.ObjectId)),x) for x in live_list)
    if len(live)!=len(live_list) or set(live)!=set((x['name'],int(x['object_id'])) for x in recorded):
        raise RuntimeError(u'警告区已有控制的名称/对象 ID 与发布记录不一致，可能被人工修改。')
    cs_live=dict(((utext(x.Name),int(x.ObjectId)),x) for x in Model.CoordinateSystems.Children)
    verified=[]
    for item in recorded:
        control=live[(item['name'],int(item['object_id']))]
        if not _is_sizing(control) or utext(getattr(control, 'Type', '')) != utext(SizingType.SphereOfInfluence):
            raise RuntimeError(u'旧自建 Sizing 类型已被人工修改，拒绝继续发布：' + item['name'])
        if _is_control_suppressed(control):
            raise RuntimeError(u'旧自建 Sizing 已被人工停用，不能安全累计；请先人工核对：' + item['name'])
        key=(item['coordinate_system'],int(item['cs_object_id']))
        cs=cs_live.get(key)
        if cs is None:
            raise RuntimeError(u'旧坐标系不存在或其对象 ID 已变化，不能自动接管。')
        if set(_effective_sizing_ids(control))!={int(item['body_id'])}:
            raise RuntimeError(u'旧 Sizing 作用域已被人工修改，拒绝继续发布。')
        if abs(_read_control_mm(control)-float(item['size_mm']))>0.001:
            raise RuntimeError(u'旧 Sizing 尺寸已被人工修改，拒绝继续发布。')
        if abs(_length_mm(control.SphereRadius)-float(item['radius_mm']))>0.001:
            raise RuntimeError(u'旧 Sizing 半径已被人工修改，拒绝继续发布。')
        if not hasattr(control,'SphereCenter') or int(control.SphereCenter.ObjectId)!=int(cs.ObjectId):
            raise RuntimeError(u'旧 Sizing 球心坐标系已被人工调整。')
        coords=_origin_coords_mm(cs)
        expected=item.get('centre_mm')
        if not isinstance(expected,list) or len(expected)!=3:
            raise RuntimeError(u'旧版没有记录球心审计数据，无法安全接管；请人工核对。')
        if any(abs(float(coords[i])-float(expected[i]))>0.001 for i in range(3)):
            raise RuntimeError(u'旧坐标系原点已被人工修改，拒绝继续发布。')
        verified.append((control,cs))
    return verified

def _reuse_owned_controls(owned, live_controls, groups, plan, managed_file, state):
    """Atomic best-effort update of documented AutoMesh-owned controls.

    Never delete existing controls; if a new plan needs a different count,
    reject rather than create leftovers or silently suppress manual changes.
    """
    if len(live_controls)!=len(groups):
        raise RuntimeError(u'候选组数变化；为防止遗留旧控制，需先人工审核现有球形 Sizing。')
    undo=[]
    entries=[]
    try:
        for i,(control,cs) in enumerate(live_controls):
            cancellation_checkpoint()
            g=groups[i]
            prev={'centre':_origin_coords_mm(cs),
                  'size':control.ElementSize,'radius':control.SphereRadius,
                  'scope':control.Location,'type':control.Type,'cs':control.SphereCenter,
                  'origin_mode':getattr(cs,'OriginDefineBy',None)}
            undo.append((control,cs,prev))
            _set_origin_mm(cs,g['centre_mm'])
            selection=ExtAPI.SelectionManager.CreateSelectionInfo(SelectionTypeEnum.GeometryEntities)
            selection.Ids=[int(g['body_id'])]
            control.Location=selection
            control.Type=SizingType.SphereOfInfluence
            control.SphereCenter=cs
            control.SphereRadius=Quantity('%s [mm]' % g['radius_mm'])
            control.ElementSize=Quantity('%s [mm]' % g['element_size_mm'])
            if set(int(v) for v in control.Location.Ids)!={int(g['body_id'])}:
                raise RuntimeError(u'复用后 Sizing 几何作用域回读失败。')
            if any(abs(x-y)>0.001 for x,y in zip(_origin_coords_mm(cs),g['centre_mm'])):
                raise RuntimeError(u'复用后球心坐标读取不一致。')
            entries.append({'name':utext(control.Name),'object_id':int(control.ObjectId),
                            'coordinate_system':utext(cs.Name),'cs_object_id':int(cs.ObjectId),
                            'body_id':g['body_id'],'radius_mm':g['radius_mm'],
                            'size_mm':g['element_size_mm'],'centre_mm':g['centre_mm']})
        write(managed_file,{'plan_id':plan['plan_id'],'project':state['project'],
                            'fingerprint':state['fingerprint'],'controls':entries,
                            'baseline_warning_count':plan['warning_count'],
                            'baseline_mesh_elements':plan['mesh_elements']})
    except Exception as original_error:
        rollback_errors=[]
        for control,cs,old in reversed(undo):
            try:
                _set_origin_mm(cs,old['centre'])
                if old['origin_mode'] is not None and hasattr(cs,'OriginDefineBy'):
                    cs.OriginDefineBy=old['origin_mode']
                control.Location=old['scope']
                control.Type=old['type']
                control.SphereCenter=old['cs']
                control.SphereRadius=old['radius']
                control.ElementSize=old['size']
            except Exception as recovery_error:
                rollback_errors.append(utext(recovery_error))
        if rollback_errors:
            raise RuntimeError(u'发布更新失败且 ACT 回退不完全；请在 Meshing 检查原 Sizing。原错误: %s；回退错误: %s' %
                               (utext(original_error),'; '.join(rollback_errors)))
        raise
    return {'status':'reused_existing','plan_id':plan['plan_id'],
            'sizing_count':len(entries),'controls':entries,
            'mesh_generated':False,'mesh_cleared':False}


def _load_operator_selection(plan, state):
    """Only an explicit, project-bound operator selection can be published."""
    filename=_warning_path(_WARNING_SELECTION)
    if not os.path.isfile(filename):
        raise RuntimeError(u'请先在 AutoMesh 候选审核列表勾选区域并确认；不允许默认发布全部候选。')
    selection=read(filename)
    if selection.get('schema_version')!=1 or selection.get('plan_id')!=plan['plan_id']:
        raise RuntimeError(u'本次勾选不属于当前已审核的方案，请重新选择。')
    if selection.get('project')!=state['project'] or selection.get('geometry_fingerprint')!=state['fingerprint']:
        raise RuntimeError(u'所选候选不属于当前工程或几何。')
    if int(selection.get('mesh_elements',-1))!=int(Model.Mesh.Elements):
        raise RuntimeError(u'选择时网格单元数量与当前不一致，拒绝应用旧候选。')
    ids=selection.get('candidate_ids') or []
    if not isinstance(ids,list) or len(ids)!=len(set(ids)) or len(ids)>MAX_SPHERES:
        raise RuntimeError(u'候选选择清单格式无效或超过上限。')
    if not ids or not all(isinstance(x,str) and len(x)==20 for x in ids):
        raise RuntimeError(u'没有明确勾选可识别的候选。')
    acknowledged=selection.get('manually_reviewed_candidate_ids') or []
    if not isinstance(acknowledged,list) or set(ids)!=set(acknowledged):
        raise RuntimeError(u'并非所有待发布影响域都经过逐项人工审核（包括晶圆间隙）。')
    return ids


def _control_proxy(control):
    r=_length_mm(control.SphereRadius)
    h=_read_control_mm(control)
    if r<=0 or h<=0 or math.isnan(r) or math.isnan(h):
        raise RuntimeError(u'已发布 Sizing 的半径/尺寸不能读取，无法审核累计资源。')
    return (4.0/3.0)*math.pi*(r/h)**3


def _candidate_record(control, cs, candidate, plan, batch_number):
    return {'candidate_id':candidate['candidate_id'],
            'name':utext(control.Name),'object_id':int(control.ObjectId),
            'coordinate_system':utext(cs.Name),'cs_object_id':int(cs.ObjectId),
            'body_id':candidate['body_id'], 'radius_mm':candidate['radius_mm'],
            'size_mm':candidate['element_size_mm'],'centre_mm':candidate['centre_mm'],
            'cell_budget_proxy':candidate['cell_budget_proxy'],
            'source_plan_id':plan.get('plan_id'),
            'source_mesh_elements':int(plan.get('mesh_elements', 0)),
            'batch_number':int(batch_number), 'published_at':time.time()}

def _migrate_legacy_manifest(owned, verified, current_candidates):
    """Bind only exact legacy matches; keep unmatched prior-round controls.

    A remesh changes element IDs and candidate IDs. A live, verified old Sizing
    therefore remains valid history even when it is absent from the new plan.
    Its actual ACT geometry is checked by _owned_live_controls and it remains
    part of the cumulative budget and duplicate-sphere checks.
    """
    changed=False
    used=set(x.get('candidate_id') for x in owned.get('controls',[]) if x.get('candidate_id'))
    for old in owned.get('controls',[]):
        if old.get('candidate_id'):
            continue
        matches=[]
        for c in current_candidates:
            if c['candidate_id'] in used:continue
            if int(old.get('body_id',-1)) != int(c['body_id']):continue
            if abs(float(old['radius_mm'])-float(c['radius_mm']))>0.001:continue
            if abs(float(old['size_mm'])-float(c['element_size_mm']))>0.001:continue
            origin=old.get('centre_mm') or []
            if len(origin)!=3 or any(abs(float(origin[k])-float(c['centre_mm'][k]))>0.001 for k in range(3)):continue
            matches.append(c)
        if len(matches)==1:
            old['candidate_id']=matches[0]['candidate_id']
            old['cell_budget_proxy']=matches[0]['cell_budget_proxy']
            used.add(old['candidate_id'])
            changed=True
    if changed:owned['schema_version']=3
    return changed

def _verify_published_geometry(owner, candidates):
    """Verify same-round ID matches; retain unmatched prior-round controls.

    Candidate IDs include marked mesh element IDs. After remeshing, new element
    IDs produce new candidate IDs. Old Sizing validity is established by
    _owned_live_controls against live ACT objects, not by membership in the
    current mesh's candidate list.
    """
    index=dict((c['candidate_id'],c) for c in candidates)
    for old in owner.get('controls',[]):
        ident=old.get('candidate_id')
        current=index.get(ident) if ident else None
        if current is None:
            continue
        if int(old['body_id']) != int(current['body_id']):
            raise RuntimeError(u'同一候选 ID 对应的几何 Body 已变化，拒绝重复或覆盖 Sizing。')
        if abs(float(old['radius_mm'])-float(current['radius_mm']))>0.001 or abs(float(old['size_mm'])-float(current['element_size_mm']))>0.001:
            raise RuntimeError(u'已发布 Sizing 对应的同一候选 ID，其尺寸/半径与当前方案不一致；几何位置可能变化，拒绝叠加发布。')
        old_coords=old.get('centre_mm') or []
        if len(old_coords)!=3 or any(abs(float(old_coords[k])-float(current['centre_mm'][k]))>0.001 for k in range(3)):
            raise RuntimeError(u'同一候选 ID 的球心与当前方案不同，拒绝重复或覆盖 Sizing。')

def _reject_near_duplicate_spheres(chosen, old_controls):
    """Protect against very close same-body spheres that are likely redundant.

    Ordinary partially overlapping SOIs are NOT automatically forbidden: the
    user must review them. Near-coincident areas are blocked before ACT changes.
    """
    spheres=[(c['body_id'],c['centre_mm'],float(c['radius_mm']),c['candidate_id']) for c in chosen]
    spheres.extend((int(c['body_id']),c.get('centre_mm',[]),float(c['radius_mm']),c.get('candidate_id','old')) for c in old_controls)
    for i in range(len(spheres)):
        a=spheres[i]
        for j in range(i+1,len(spheres)):
            b=spheres[j]
            if a[0]!=b[0] or len(a[1])!=3 or len(b[1])!=3:
                continue
            if _distance(a[1],b[1]) < .25*min(a[2],b[2]):
                raise RuntimeError(u'所选区域与现有或同批球形影响域几乎重合（%s, %s），可能重复细化。请只保留其中一个或人工调整方案。' % (a[3],b[3]))


def _check_budget_warning_approval(cumulative_proxy, receipt):
    """ACT-side independent acknowledgement for a verified live proxy.

    Return True only for operator-approved over-threshold publication. Never
    infer consent from an archived plan, old receipt or a near-120000 value.
    """
    if not isinstance(cumulative_proxy,(int,float)) or math.isnan(cumulative_proxy) or math.isinf(cumulative_proxy) or cumulative_proxy<0:
        raise RuntimeError(u'当前 ACT 预算代理值异常，拒绝发布。')
    if cumulative_proxy<=MAX_CELL_PROXY:
        return False
    if not isinstance(receipt,dict) or receipt.get('budget_warning_acknowledged') is not True:
        raise RuntimeError(u'实际 ACT 预算代理 %.0f 超过建议预警值 %.0f，缺少本轮额外人工确认；未创建 Sizing。' %
                           (cumulative_proxy, MAX_CELL_PROXY))
    return True


def publish_warning_refinement(state):
    """One operator-approved selection; 6 controls per transactional batch.

    No unselected candidate can become Sizing. Completed batches are recorded
    durably; uncertain ACT state never triggers automatic retry or cleanup.
    """
    cancellation_checkpoint()
    fname=_warning_path(_WARNING_PLAN)
    if not os.path.isfile(fname):
        raise RuntimeError(u'尚无已审核的候选方案。')
    plan=read(fname)
    if plan.get('status')!='ready' or not plan.get('plan_id'):
        raise RuntimeError(u'候选方案不存在或已失效，没有可发布的局部细化建议；请重新生成。')
    if plan.get('project')!=state['project'] or plan.get('geometry_fingerprint')!=state['fingerprint']:
        raise RuntimeError(u'工程或几何变化，拒绝应用过期候选。')
    _, observations, digest=_read_warning_elements(state)
    if digest!=plan.get('warning_digest') or int(Model.Mesh.Elements)!=int(plan['mesh_elements']):
        raise RuntimeError(u'目标单元节点、偏度数据或网格标记发生变化，请重新分析。')
    # Recompute without any active-selection budget to prove the candidate
    # geometry and ordering are identical to the approved read-only preview.
    caps=_existing_body_size_caps(state)
    all_candidates,_=_hotspot_candidates(observations,caps,fingerprint=state['fingerprint'],metric=plan.get('quality_metric','Skewness'))
    if all_candidates!=plan.get('clusters'):
        raise RuntimeError(u'候选几何或质量值与预览不一致，禁止使用旧审核结果。')
    chosen_ids=_load_operator_selection(plan,state)
    index=dict((x['candidate_id'],x) for x in all_candidates)
    if len(index)!=len(all_candidates) or any(i not in index for i in chosen_ids):
        raise RuntimeError(u'候选身份冲突或选中区域不属于当前有效方案。')
    chosen=[index[i] for i in chosen_ids]
    if not hasattr(Model,'CoordinateSystems') or not hasattr(Model.CoordinateSystems,'AddCoordinateSystem'):
        raise RuntimeError(u'ACT 无法创建坐标系。')
    if not hasattr(Model.Mesh,'AddSizing') or not hasattr(SizingType,'SphereOfInfluence'):
        raise RuntimeError(u'ACT 无法创建球形 Sizing。')
    valid=set(int(b['id']) for b in state['bodies'])
    for c in chosen:
        if c['body_id'] not in valid or not (0.5<=c['element_size_mm']<=5.0) or c['radius_mm']>MAX_RADIUS_MM:
            raise RuntimeError(u'候选几何、半径或单元尺寸未通过安全复核。')
    managed_file=_warning_path(_WARNING_MANAGED)
    owner=_current_warning_owner(state)
    existing=_live_warning_qr_controls()
    live=_owned_live_controls(owner,existing)
    if existing:
        _migrate_legacy_manifest(owner,live,all_candidates)
        _verify_published_geometry(owner,all_candidates)
    elif owner.get('controls'):
        raise RuntimeError(u'警告区发布清单存在但原 Sizing 已不在当前模型树中，禁止忽略历史记录。')
    # Abandoned AutoMesh coordinate systems are treated as manual-recovery
    # evidence; never delete them automatically.
    known_cs=set((e.get('coordinate_system'),int(e.get('cs_object_id',-1))) for e in owner.get('controls',[]))
    for cs in Model.CoordinateSystems.Children:
        if utext(cs.Name).startswith('AutoMesh_QR_CS_') and (utext(cs.Name),int(cs.ObjectId)) not in known_cs:
            raise RuntimeError(u'发现未归属的 AutoMesh 球心坐标系，请先人工核查。')
    published=set(e.get('candidate_id') for e in owner.get('controls',[]) if e.get('candidate_id'))
    if any(i in published for i in chosen_ids):
        raise RuntimeError(u'所勾选区域包含已发布 Sizing 的候选；请取消该勾选，不能重复创建。')
    # Cumulative budget: all existing AutoMesh controls plus proposed new ones.
    old_budget=sum(_control_proxy(control) for control,cs in live)
    new_budget=sum(float(c['cell_budget_proxy']) for c in chosen)
    _reject_near_duplicate_spheres(chosen,owner.get('controls',[]))
    cumulative_proxy=old_budget+new_budget
    # Re-read operator receipt after verifying live ACT controls. The old
    # schema stays compatible below threshold, but old receipts MUST NOT
    # silently authorize over-threshold publication.
    budget_receipt=read(_warning_path(_WARNING_SELECTION))
    budget_ack=_check_budget_warning_approval(cumulative_proxy,budget_receipt)
    preflight=_warning_path('warning_publish_preflight.json')
    write(preflight,{'plan_id':plan['plan_id'],'selected_candidate_ids':chosen_ids,
                     'selected_count':len(chosen),'cumulative_cell_budget_proxy':round(old_budget+new_budget),
                     'status':'preflight_ok','budget_warning':cumulative_proxy>MAX_CELL_PROXY,
                     'budget_warning_acknowledged':budget_ack,
                     'mesh_elements':int(Model.Mesh.Elements)})
    # Mutation begins only after all constraints and filesystem checks pass.
    if not owner:
        owner={'schema_version':3,'project':state['project'],'fingerprint':state['fingerprint'],
               'baseline_mesh_elements':int(Model.Mesh.Elements),'controls':[],'rounds':[]}
    owner['schema_version']=3
    owner['project']=state['project']
    owner['fingerprint']=state['fingerprint']
    owner['plan_id']=plan['plan_id']
    owner['last_mesh_elements']=int(Model.Mesh.Elements)
    rounds=list(owner.get('rounds',[]))
    round_entry=None
    for entry in rounds:
        if entry.get('plan_id')==plan['plan_id']:
            round_entry=entry
            break
    if round_entry is None:
        current_plan_ids=set(x['candidate_id'] for x in all_candidates)
        prior_current_ids=sorted(set(x.get('candidate_id') for x in owner.get('controls',[])
                                     if x.get('candidate_id') in current_plan_ids))
        round_entry={'plan_id':plan['plan_id'],'mesh_elements':int(Model.Mesh.Elements),
                     'published_candidate_ids':prior_current_ids,'published_count':len(prior_current_ids),
                     'created_at':time.time(),'continued_from_legacy_record':bool(prior_current_ids)}
        rounds.append(round_entry)
    owner['rounds']=rounds
    previous=len(owner['controls'])
    total_batches=(len(chosen)+MAX_BATCH_SPHERES-1)//MAX_BATCH_SPHERES
    for batch_start in range(0,len(chosen),MAX_BATCH_SPHERES):
        cancellation_checkpoint()
        subset=chosen[batch_start:batch_start+MAX_BATCH_SPHERES]
        batch_no=batch_start//MAX_BATCH_SPHERES+1
        report=globals().get('emit_progress')
        if report:
            report(None,'分批发布所选 Sizing','第 %d/%d 批；本批 %d 个' % (batch_no,total_batches,len(subset)))
        made=[]; batch_entries=[]
        try:
            for i,c in enumerate(subset):
                cancellation_checkpoint()
                stem='AutoMesh_QR_%s' % c['candidate_id'][:12]
                if any(utext(x.Name)==stem for x in Model.Mesh.Children):
                    raise RuntimeError(u'命名冲突：'+stem)
                cs=Model.CoordinateSystems.AddCoordinateSystem()
                made.append(cs)
                cs.Name='AutoMesh_QR_CS_%s' % c['candidate_id'][:12]
                _set_origin_mm(cs,c['centre_mm'])
                sizing=Model.Mesh.AddSizing()
                made.append(sizing)
                sizing.Name=stem
                scope=ExtAPI.SelectionManager.CreateSelectionInfo(SelectionTypeEnum.GeometryEntities)
                scope.Ids=[int(c['body_id'])]
                sizing.Location=scope
                sizing.Type=SizingType.SphereOfInfluence
                sizing.SphereCenter=cs
                sizing.SphereRadius=Quantity('%s [mm]' % c['radius_mm'])
                sizing.ElementSize=Quantity('%s [mm]' % c['element_size_mm'])
                if utext(sizing.Type)!=utext(SizingType.SphereOfInfluence):
                    raise RuntimeError(u'球形控制类型回读失败。')
                if set(int(v) for v in sizing.Location.Ids)!={int(c['body_id'])}:
                    raise RuntimeError(u'球形控制 Body 作用域回读失败。')
                if abs(_read_control_mm(sizing)-float(c['element_size_mm']))>0.001:
                    raise RuntimeError(u'球形控制尺寸回读失败。')
                if abs(_length_mm(sizing.SphereRadius)-float(c['radius_mm']))>0.001:
                    raise RuntimeError(u'球形控制半径回读失败。')
                if any(abs(x-y)>0.001 for x,y in zip(_origin_coords_mm(cs),c['centre_mm'])):
                    raise RuntimeError(u'球心坐标回读失败。')
                batch_entries.append(_candidate_record(sizing,cs,c,plan,batch_no))
            # Persist the entire committed ledger AFTER ACT verifies every
            # object in this batch; earlier batches remain traceable on error.
            amended=dict(owner)
            amended['controls']=owner['controls']+batch_entries
            amended_rounds=[]
            for item in owner.get('rounds',[]):
                item_copy=dict(item)
                if item_copy.get('plan_id')==plan['plan_id']:
                    prior_ids=list(item_copy.get('published_candidate_ids',[]))
                    prior_ids.extend(x['candidate_id'] for x in batch_entries)
                    item_copy['published_candidate_ids']=prior_ids
                    item_copy['published_count']=len(prior_ids)
                    item_copy['updated_at']=time.time()
                amended_rounds.append(item_copy)
            amended['rounds']=amended_rounds
            amended['updated_at']=time.time()
            write(managed_file,amended)
            owner=amended
        except Exception as original_error:
            cleanup_errors=[]
            for obj in reversed(made):
                try: obj.Delete()
                except Exception as cleanup_error:cleanup_errors.append(utext(cleanup_error))
            if cleanup_errors:
                try:
                    write(_warning_path('warning_publish_partial.json'),{
                        'plan_id':plan['plan_id'],'status':'manual_reconciliation_required',
                        'committed_count':len(owner['controls'])-previous,
                        'selected_candidate_ids':chosen_ids,'batch_number':batch_no,
                        'original_error':utext(original_error),'cleanup_errors':cleanup_errors})
                except Exception:pass
                raise RuntimeError(u'批次 %d 发布失败且回退不完整，已保留现场，禁止重试；请核查 Meshing。原因: %s' % (batch_no,utext(original_error)))
            raise RuntimeError(u'第 %d 批发布失败，已回滚本批；此前成功的 %d 个候选保留记录。原因: %s' %
                               (batch_no,len(owner['controls'])-previous,utext(original_error)))
    current_plan_ids=set(x['candidate_id'] for x in all_candidates)
    current_round_published=set(x.get('candidate_id') for x in owner.get('controls',[])
                                 if x.get('candidate_id') in current_plan_ids)
    remaining=sum(1 for x in all_candidates if x['candidate_id'] not in current_round_published)
    snapshot_refresh_warning=''
    try:
        state['mesh'] = state.get('mesh') or {}
        for field in ('Elements','Nodes','ElementSize','MaximumSize','GrowthRate'):
            try: state['mesh'][field] = utext(getattr(Model.Mesh, field))
            except Exception: pass
        state.update(live_control_inventory())
        write(SNAPSHOT_FILE, state)
    except Exception as snapshot_error:
        # The Sizing ledger is already committed. A cache-refresh failure must
        # not report the ACT publication as failed or invite a duplicate retry.
        snapshot_refresh_warning=u'Sizing 已提交，但本地只读快照刷新失败；下一次只读任务会重新读取：'+utext(snapshot_error)
    return {'status':'published','plan_id':plan['plan_id'],
            'sizing_count':len(chosen),'batch_count':total_batches,
            'candidate_ids':chosen_ids,'controls':owner['controls'][-len(chosen):],
            'existing_control_count':len(owner.get('controls',[]))-len(chosen),
            'cumulative_control_count':len(owner.get('controls',[])),
            'remaining_count':remaining,'mesh_generated':False,'mesh_cleared':False,
            'snapshot_refresh_warning':snapshot_refresh_warning,
            'next_action':u'本轮未勾选候选仍可继续审核；如已重新生成网格，请重新标记偏度并生成新方案。历史 Sizing 会保留、核验并计入累计预算。'}

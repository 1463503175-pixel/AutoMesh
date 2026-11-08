# -*- coding: utf-8 -*-
"""Pure, conservative meshing feature heuristics (Python 2.7/3 compatible).

All geometry lengths in snapshot are converted to millimeters by CAD unit factor.
Every feature type is a geometric descriptor, NOT a boundary condition inference.
These are surface-size hints, not guaranteed mesh counts or CFD validity.
"""
import math
import re
from bisect import bisect_left, bisect_right

try:
    _unicode_type = unicode
except NameError:
    _unicode_type = str

def safe_text(value):
    if isinstance(value, _unicode_type): return value
    if isinstance(value, bytes): return value.decode('utf-8', 'replace')
    return _unicode_type(value)

_UNIT_FACTORS = {
    'standardmks': 1000.0, 'standardnmm': 1.0,
    'standardumks': 0.001, 'standardbin': 25.4,
    'standardbft': 304.8, 'standardcgs': 10.0,
    'm': 1000.0, 'meter': 1000.0, 'metre': 1000.0,
    'mm': 1.0, 'millimeter': 1.0, 'millimetre': 1.0,
    'cm': 10.0, 'centimeter': 10.0,
    'um': 0.001, 'micrometer': 0.001,
    'in': 25.4, 'inch': 25.4, 'ft': 304.8, 'feet': 304.8,
}

def unit_to_mm(unit):
    # Explicit whitelist: never infer unsupported units.
    key = str(unit).strip().lower()
    direct=_UNIT_FACTORS.get(key)
    if direct is not None:return direct
    # Some ACT releases stringify enums with a fully qualified namespace.
    # Only accept an explicitly whitelisted final enum token.
    if '.' in key:
        return _UNIT_FACTORS.get(key.rsplit('.',1)[-1])
    return None


def dot(a, b):
    return sum(a[i]*b[i] for i in range(3))


def norm(a):
    x = math.sqrt(dot(a,a))
    return [v/x for v in a] if x > 1.e-12 else None


def spatially_partition(groups, faces, factor, max_groups=32):
    """Bounded spatial partitions for reviewable geometry selections.

    Split only substantial repeated groups, by a detected coordinate axis.
    Stable region keys are descriptive, not evidence of functional identity.
    """
    meta={int(f['id']):f for f in faces}
    out=[]
    for gr in groups:
        if gr['category'] not in ('circular_cylindrical','narrow_parallel'):
            out.append(gr)
            continue
        bybody={}
        for i in gr['ids']:
            f=meta.get(int(i))
            if not f:
                continue
            bybody.setdefault(str(f.get('body','unknown')),[]).append(int(i))
        # Do not fan out thousands of groups across a complex imported model.
        if not bybody or len(bybody)>8:
            g=dict(gr)
            g['region_key']='G%d' % (len(out)+1)
            out.append(g)
            continue
        for body in sorted(bybody):
            ids=sorted(set(bybody[body]))
            with_center=[(fid,meta[fid].get('centroid')) for fid in ids]
            with_center=[(fid,tuple(float(v)*factor for v in c)) for fid,c in with_center
                         if isinstance(c,(tuple,list)) and len(c)==3]
            # Split only at a real spatial break. Median splitting a continuous
            # hole array produces arbitrary halves that may cross plate borders.
            split=False
            if len(ids)>=48 and len(with_center)==len(ids):
                spans=[max(c[k] for _,c in with_center)-min(c[k] for _,c in with_center) for k in range(3)]
                axis=max(range(3),key=lambda k:spans[k])
                ordered=sorted(with_center,key=lambda item:(item[1][axis],item[0]))
                separations=[(ordered[i][1][axis]-ordered[i-1][1][axis],i)
                             for i in range(24,len(ordered)-23)]
                if separations:
                    gap,index=max(separations)
                    # Only separate spatially disjoint clusters, never by index
                    # alone. This remains a geometric grouping, not a CAD part ID.
                    if gap>max(20.0,4.0*gr.get('mm',1.0)) and len(out)+2<=max_groups:
                        bins=[sorted(x[0] for x in ordered[:index]),
                              sorted(x[0] for x in ordered[index:])]
                        for part,these in enumerate(bins):
                            g=dict(gr);g['ids']=these
                            g['region_key']='B%s_A%d_C%d' % (body,axis,part+1)
                            g['spatial_split_axis']=axis
                            g['spatial_split_gap_mm']=round(gap,2)
                            out.append(g)
                        split=True
            if not split:
                g=dict(gr);g['ids']=ids;g['region_key']='B%s' % body
                out.append(g)
    # Keep width bands distinct after positional splitting to avoid duplicate AM_* names.
    for gr in out:
        if gr['category']=='narrow_parallel':
            orient=gr.get('gap_orientation',(0,0,0))
            orient_key='_'.join(str(int(v)).replace('-','N') for v in orient)
            gr['region_key']='W%s_O%s_%s' % (gr.get('gap_width_band','X'),orient_key,gr.get('region_key','G'))
    # The original cylinders are already sorted into deterministic diameter bands.
    for gr in out:
        if gr['category']=='circular_cylindrical':
            gr['region_key']='D%s_%s' % (str(gr.get('diameter_band','X')).replace('-','N'),gr.get('region_key','G'))
    return out


def known_curved_groups(faces, factor, base_mm):
    """Analytic non-cylindrical curved surfaces: geometric grouping only."""
    types=('spher','conic','cone','toroid')
    buckets={}
    for f in faces:
        kind=str(f.get('type','')).lower()
        matched=next((k for k in types if k in kind),None)
        if not matched:continue
        try:
            area=float(f['area'])*factor*factor
            if not (16.0<=area<=10000000.0):continue
            band=int(math.floor(math.log(max(area,1.0),4.0)))
            key=(matched,band,str(f.get('body','unknown')))
            buckets.setdefault(key,[]).append(int(f['id']))
        except (ValueError,TypeError,OverflowError,KeyError):continue
    if len(buckets)>12:
        return []  # report via type histogram instead of unsafe mass grouping
    groups=[]
    for key in sorted(buckets):
        ids=sorted(set(buckets[key]))
        if len(ids)>1200:continue
        groups.append({'category':'curved_surface','ids':ids,'region_key':'%s_A%d_B%s' % key,
                       'mm':round(max(0.6,min(base_mm*0.6,3.5)),3),
                       'confidence':'low','native_function':'curvature',
                       'reason':'Analytic curved face family; manual confirmation required'})
    return groups


def classify(state, base_mm, max_faces=90000, max_gap_pairs=12000):
    if not 0.5 <= base_mm <= 500.0:
        raise ValueError('Reference size outside 0.5-500 mm')
    unit = state.get('geometry_unit','')
    factor = unit_to_mm(unit)
    if not factor:
        raise ValueError('Unknown CAD geometry length unit %r. Do not guess meters.' % unit)
    faces = state.get('faces', [])
    if not faces or len(faces) > max_faces:
        raise ValueError('Invalid or excessive geometry face count; abort sizing')
    evidence = {
        'geometry_unit':unit, 'unit_to_mm':factor, 'face_count':len(faces),
        'circular_features':0, 'narrow_parallel_pairs':0,
        'gap_mm_min':None, 'gap_detection':'validated_centroid_normal_and_parallel_overlap',
        'boundary_roles':'explicit_named_selections_only', 'named_region_groups':0, 'warnings':[], 'samples':[],
    }
    # Candidate circular cylindrical faces: require at least two comparable full-circle edges.
    cylinders = []  # tuples of (id, verified diameter_mm)
    circular_sizes = []
    planar = []
    for f in faces:
        typ = str(f.get('type','')).lower()
        if 'cylind' in typ:
            rings = sorted([float(r)*factor*2.0 for r in f.get('full_circle_radii', [])
                            if isinstance(r,(int,float)) and r > 0])
            for i in range(len(rings)-1):
                diameter = rings[i]
                if (0.4 <= diameter <= max(2.5*base_mm, 3.0)
                        and abs(rings[i+1]-diameter) < 0.03*diameter):
                    cylinders.append((f['id'], diameter))
                    circular_sizes.append(diameter)
                    evidence['samples'].append({'kind':'circular_cylindrical', 'id':f['id'],
                                                'diameter_mm':round(diameter,2)})
                    break
        if ('plan' in typ or 'plane' in typ) and all(k in f for k in ('centroid','normal','bbox')):
            try:
                c = [float(x)*factor for x in f['centroid']]
                n = norm([float(x) for x in f['normal']])
                box = [float(x)*factor for x in f['bbox']]
                area = float(f['area'])*factor*factor
                if n is not None and len(box)==6 and area >= max(16,base_mm*base_mm):
                    planar.append({'id':f['id'],'body':f.get('body'),
                                   'c':c,'n':n,'box':box,'a':area})
            except (ValueError,TypeError,OverflowError):
                pass
    groups = []
    if cylinders:
        # Keep distinct diameter bands separate: a small inlet or baffle orifice
        # must not force every large port to use the same fine element size.
        # Ratio bands are deterministic across repeated scans, and do not infer
        # actual physical port/wafer roles from cylinder shape.
        diameter_bands = {}
        for face_id, diameter in cylinders:
            band = int(math.floor(math.log(diameter, 1.6)))
            diameter_bands.setdefault(band, []).append((face_id, diameter))
        for band in sorted(diameter_bands):
            entries = diameter_bands[band]
            diameters = [v for _, v in entries]
            smallest = min(diameters)
            size = max(0.6, min(base_mm * 0.6, smallest / 3.0))
            groups.append({'category':'circular_cylindrical',
                'diameter_band':band, 'diameter_mm_min':round(smallest, 3),
                'diameter_mm_max':round(max(diameters), 3),
                'ids':sorted(set(fid for fid, _ in entries)),
                'mm':round(size,3),'confidence':'medium','native_function':'curvature',
                'reason':'Diameter-specific cylindrical face group (not a proven through-hole)'})
        evidence['cylinder_diameter_groups'] = [
            {'diameter_mm_min':g['diameter_mm_min'],
             'diameter_mm_max':g['diameter_mm_max'],
             'face_count':len(g['ids']), 'default_mm':g['mm']}
            for g in groups if g['category']=='circular_cylindrical']
    else:
        evidence['cylinder_diameter_groups'] = []
    evidence['circular_features'] = len(set(fid for fid, _ in cylinders))

    # Plane indexing: orientation + coarse in-plane centroid tiles. Unlike
    # an all-pairs search, this stays bounded on multi-boat ALD geometries.
    # The candidates are geometric opposing faces, not proof of an open flow gap.
    buckets = {}
    gap_upper = max(0.8, min(12.0, base_mm*1.25))
    tile = max(8.0, min(100.0, 4.0*base_mm))
    for f in planar:
        n = f['n']
        axis = max(range(3), key=lambda k:abs(n[k]))
        sign = 1 if n[axis] >= 0 else -1
        canonical = [v*sign for v in n]
        trans = [k for k in range(3) if k != axis]
        orient = tuple(int(round(v*20)) for v in canonical)
        key = (orient, int(round(f['c'][trans[0]]/tile)), int(round(f['c'][trans[1]]/tile)), f['body'])
        f['projection'] = dot(f['c'], canonical)
        f['orientation'] = orient
        buckets.setdefault(key, []).append(f)
    # Adjacent tiles must also be checked: two coincident long wafer faces
    # can have centroids on opposite sides of a tile boundary. Sorted 1D
    # projections + bounded neighbor tiles prevent an all-pairs O(N^2) scan.
    for items in buckets.values():
        items.sort(key=lambda f:f['projection'])
    projections={key:[f['projection'] for f in items] for key,items in buckets.items()}
    pairs=[]
    seen=set()
    capped=False
    comparisons=0
    for key in sorted(buckets, key=lambda x:str(x)):
        for a in buckets[key]:
            if capped:break
            for dx in (-1,0,1):
                if capped:break
                for dy in (-1,0,1):
                    nkey=(key[0],key[1]+dx,key[2]+dy,key[3])
                    items=buckets.get(nkey)
                    if not items:continue
                    proj=projections[nkey]
                    lower=bisect_left(proj,a['projection']-gap_upper)
                    upper=bisect_right(proj,a['projection']+gap_upper)
                    if upper-lower>128:
                        # Explicit loss of coverage; prevents pathological dense
                        # clusters from turning a quick scan into minutes of work.
                        capped=True
                        break
                    for b in items[lower:upper]:
                        if b['id']<=a['id']:continue
                        comparisons+=1
                        delta=[b['c'][k]-a['c'][k] for k in range(3)]
                        gap=abs(dot(delta,a['n']))
                        if not (0.35<=gap<=gap_upper):continue
                        if dot(a['n'],b['n'])>-0.985:continue
                        lateral=math.sqrt(max(0.0,dot(delta,delta)-gap*gap))
                        if lateral>0.25*math.sqrt(min(a['a'],b['a'])):continue
                        if max(a['a'],b['a'])>1.5*min(a['a'],b['a']):continue
                        axis=max(range(3),key=lambda k:abs(a['n'][k]))
                        overlap=True
                        for k in range(3):
                            if k==axis:continue
                            width=min(a['box'][k+3],b['box'][k+3])-max(a['box'][k],b['box'][k])
                            extent=max(1.e-8,min(a['box'][k+3]-a['box'][k],b['box'][k+3]-b['box'][k]))
                            if width<0.45*extent:
                                overlap=False
                                break
                        if not overlap:continue
                        ids=(min(a['id'],b['id']),max(a['id'],b['id']))
                        if ids in seen:continue
                        seen.add(ids)
                        pairs.append({'gap':gap,'ids':ids,'orientation':a['orientation'],
                                      'area':min(a['a'],b['a']),'body':a['body']})
                        if len(pairs)>=max_gap_pairs:
                            capped=True
                            break
                    if capped:break
        if capped:break
    evidence['gap_candidate_comparisons']=comparisons
    evidence['gap_tile_neighborhood']=9
    evidence['narrow_parallel_pairs'] = len(pairs)
    evidence['gap_detection'] = 'opposing_normals_bbox_overlap_9_neighbor_tiles_candidate'
    evidence['scan_truncated'] = capped
    if capped:
        evidence['warnings'].append('Gap candidates capped; full face-to-face coverage NOT guaranteed')
    if pairs:
        pairs.sort(key=lambda p:p['gap'])
        min_gap = pairs[0]['gap']
        evidence['gap_mm_min'] = round(min_gap,4)
        evidence['samples'].append({'kind':'opposing_face_candidate',
            'face_ids':list(pairs[0]['ids']), 'gap_mm':round(min_gap,4)})
        # Cluster similar gap widths and normal directions. Repeated pairs
        # are likely channel arrays; their fluid connectivity remains unknown.
        periodic={}
        for p0 in pairs:
            buck = (p0['orientation'],int(round(p0['gap']/0.25)),
                    int(round(math.log(max(p0['area'],1.0), 1.5))))
            periodic.setdefault(buck, []).append(p0)
        stacks=[]
        for pack in periodic.values():
            if len(pack)>=8:
                widths=sorted(p0['gap'] for p0 in pack)
                stacks.append({'pair_count':len(pack),'gap_mm':round(widths[len(widths)//2],3),
                               'sample_face_ids':list(pack[0]['ids']),
                               'interpretation':'repeated opposing passage candidate; wafer status unconfirmed'})
        stacks.sort(key=lambda x:-x['pair_count'])
        evidence['repeated_passage_groups']=len(stacks)
        evidence['repeated_passage_pairs']=sum(x['pair_count'] for x in stacks)
        evidence['repeated_passage_samples']=stacks[:12]
        # Width-specific groups. A single sub-resolution sliver may NOT
        # suppress the ordinary (e.g. 3mm wafer-like) passage group.
        # Geometric group != proof of a connected flow channel.
        width_bin_mm = max(0.25, min(0.5, base_mm * 0.025))
        candidate_bins = {}
        for p0 in pairs:
            key=(int(round(p0['gap']/width_bin_mm)), p0['orientation'], str(p0['body']))
            candidate_bins.setdefault(key, []).append(p0)
        area_by_id={f['id']:f['a'] for f in planar}
        accepted_ids=set()
        protected_ids=set() # tighter candidate faces cannot be reassigned to wider false gaps
        risk_proxy=0  # all candidate groups, including withheld
        accepted_proxy=0  # only accepted groups consume automatic budget
        withheld=[]
        detected=[]
        # Prefer the smallest physical separation; a wider opposite face
        # pairing must not reclaim a withheld tighter channel's faces.
        ordered=sorted(candidate_bins.items(), key=lambda x:(min(p['gap'] for p in x[1]),-len(x[1]),str(x[0])))
        for (band,orient,body), cluster in ordered:
            widths=sorted(p0['gap'] for p0 in cluster)
            local_min=widths[0]
            ids=sorted(set(fid for pair in cluster for fid in pair['ids'])-protected_ids)
            if not ids:continue
            protected_ids.update(ids)
            suggested=min(base_mm*0.55,local_min/2.5)
            entry={'gap_mm_min':round(local_min,4),'gap_mm_median':round(widths[len(widths)//2],3),
                   'pair_count':len(cluster),'face_count':len(ids),'width_band':band}
            detected.append(entry)
            if suggested<0.35:
                withheld.append(dict(entry,reason='below_minimum_safe_sizing'))
                continue
            if capped:
                withheld.append(dict(entry,reason='truncated_gap_search'))
                continue
            if len(ids)>1800:
                withheld.append(dict(entry,reason='too_many_faces'))
                continue
            surface_proxy=int(math.ceil(sum(area_by_id[i]/(suggested*suggested) for i in ids)))
            risk_proxy+=surface_proxy
            if surface_proxy>1500000 or accepted_proxy+surface_proxy>1500000:
                withheld.append(dict(entry,reason='32GB_surface_proxy_exceeded',proxy=surface_proxy))
                continue
            if len([g for g in groups if g['category']=='narrow_parallel'])>=32:
                withheld.append(dict(entry,reason='more_than_32_gap_groups'))
                continue
            groups.append({'category':'narrow_parallel','ids':ids,'gap_width_band':band,
                           'region_key':'W%s_B%s' % (band,body),'gap_orientation':orient,
                           'mm':round(suggested,4),'confidence':'medium',
                           'native_function':'proximity','gap_mm_min':round(local_min,4),
                           'gap_mm_median':entry['gap_mm_median'],'cells_per_gap':2.5,
                           'reason':'Width-specific opposing planar faces; connectivity and wafer role unproven'})
            accepted_ids.update(ids)
            accepted_proxy+=surface_proxy
        evidence['accepted_gap_surface_proxy']=accepted_proxy
        evidence['gap_width_groups']=detected[:60]
        evidence['gap_withheld_groups']=withheld[:60]
        evidence['estimated_surface_elements_proxy']=risk_proxy
        evidence['surface_proxy_is_not_volume_mesh']=True
        if any(x['reason']=='32GB_surface_proxy_exceeded' for x in withheld):
            evidence['warnings'].append('32GB risk: selected gap-width groups withheld (surface proxy > 1.5M); review individually')
        if any(x['reason']=='below_minimum_safe_sizing' for x in withheld):
            evidence['warnings'].append('Sub-resolution gap candidates withheld individually; other gap-width groups retained')
        if any(x['reason']=='too_many_faces' for x in withheld):
            evidence['warnings'].append('Large gap-width groups withheld; inspect before sizing')
        if capped:
            evidence['warnings'].append('Coverage incomplete: gap sizing withheld')
    else:
        evidence['repeated_passage_groups']=0
        evidence['repeated_passage_pairs']=0
        evidence['repeated_passage_samples']=[]
        evidence['gap_width_groups']=[]
        evidence['gap_withheld_groups']=[]
    # Never extrapolate these heuristics into mesh-quality or BC claims.
    evidence['samples'] = evidence['samples'][:24]
    # Trust explicit user Named Selections as hints; DO NOT infer boundary condition
    # semantics solely from the shape of holes or exterior faces.
    named_roles = [(re.compile(u'(^|[^a-z])(inlet|entry)([^a-z]|$)|入口|进气', re.I),'named_inlet'),
                   (re.compile(u'(^|[^a-z])(outlet|exit)([^a-z]|$)|出口|排气', re.I),'named_outlet')]
    valid_face_ids = set(int(f['id']) for f in faces)
    named_groups=[]
    for n in state.get('named', []):
        role=None
        for pattern,name in named_roles:
            if pattern.search(safe_text(n.get('name',u''))):
                role=name
                break
        if role is None:
            continue
        ids=sorted(set(int(i) for i in n.get('ids', []) if int(i) in valid_face_ids))
        if not ids or len(ids)>2400:
            evidence['warnings'].append('Named region has no valid face scope, or is too large')
            continue
        named_groups.append({'category':role,'ids':ids,
                             'mm':round(max(0.6,base_mm*0.33),3),
                             'native_function':'uniform','confidence':'named',
                             'reason':'Boundary role supplied by existing user Named Selection'})
    # Publish smaller editable regions instead of one monolithic hole/gap group.
    # Geometric location is not evidence of physical meaning; use axis-labelled
    # positions, never "top baffle" or "wafer" without manual confirmation.
    groups = spatially_partition(groups, faces, factor)
    evidence['editable_geometry_groups'] = len(groups)
    # Known curved analytic surfaces can be selected independently. Unspecified
    # freeform surfaces remain review-only; do not imply topology or flow role.
    curved = known_curved_groups(faces, factor, base_mm)
    groups.extend(curved)
    evidence['curved_group_count'] = len(curved)
    evidence['named_region_groups']=len(named_groups)
    groups=named_groups+groups
    # Stable semantic key for per-group overrides. These keys are NOT face IDs;
    # a changed CAD must still be scanned and reviewed before reusing them.
    seen_keys=set()
    for gr in groups:
        root=gr['category']+':'+safe_text(gr.get('region_key','F%s'%min(gr['ids'])))
        key=root
        n=2
        while key in seen_keys:
            key=root+'_dup%d'%n;n+=1
        gr['group_key']=key
        seen_keys.add(key)
    evidence['auto_sized_faces'] = len(set(x for g in groups for x in g['ids']))
    evidence['feature_face_fraction'] = round(float(evidence['auto_sized_faces'])/len(faces),4)
    evidence['native_proximity_candidate_count']=sum(1 for g in groups if g.get('native_function')=='proximity')
    evidence['native_curvature_candidate_count']=sum(1 for g in groups if g.get('native_function')=='curvature')
    if evidence['auto_sized_faces']==0:
        evidence['warnings'].append('No trusted geometry sizing candidates; use existing meshing defaults or inspect the model')
    return groups, evidence


def evaluate_refinement_budget(state, groups, overrides=None, disabled=None,
                               excluded_ids=None, max_surface_proxy=1500000,
                               base_mm=None, enforce_global=False,
                               max_volume_proxy=12000000, group_overrides=None,
                               disabled_groups=None, enabled_groups=None):
    """Preflight actual requested refinement sizes, not only scan defaults.

    The area/size^2 surface-element proxy is an empirical *veto* for 32 GiB
    workstations. It is NOT a volume cell count, memory estimate or guarantee.
    Overlapping Sizing controls use the smallest known element size for each
    face, never the first group encountered. This avoids underestimating
    a second, finer manual sizing. Runs before any AddSizing/GenerateMesh.
    """
    overrides = overrides or {}
    group_overrides = group_overrides or {}
    disabled_groups = set(disabled_groups or [])
    enabled_groups = set(enabled_groups or [])
    disabled = set(disabled or [])
    excluded = set(excluded_ids or [])
    effective_sizes={}
    factor = unit_to_mm(state.get('geometry_unit', ''))
    if factor is None:
        raise ValueError('Unknown geometry unit; resource check cannot run')
    areas = {}
    for f in state.get('faces', []):
        try:
            area = float(f['area']) * factor * factor
            if math.isnan(area) or math.isinf(area) or area < 0:
                raise ValueError()
            areas[int(f['id'])] = area
        except (ValueError, KeyError, TypeError, OverflowError):
            pass
    total = 0.0
    details = []
    for group in groups:
        category = group['category']
        if (category in disabled and group.get('group_key') not in enabled_groups) or group.get('group_key') in disabled_groups:
            continue
        size = float(group_overrides.get(group.get('group_key'), overrides.get(category, group['mm'])))
        if math.isnan(size) or math.isinf(size) or not 0.35 <= size <= 500:
            raise ValueError('Invalid requested local size for %s' % category)
        # Never replace the minimum two-and-a-half cells across a narrow gap
        # with a coarser user override. User can explicitly disable the group.
        if category == 'narrow_parallel':
            gap = state.get('minimum_narrow_gap_mm')
            if gap is None:
                gap = group.get('gap_mm_min')
            if gap is not None and size > float(gap)/2.5 + 1.e-5:
                raise ValueError('Requested gap size %.3f mm is too coarse for %.3f mm gap; disable gap refinement or choose <= %.3f mm' %
                                 (size, float(gap), float(gap)/2.5))
        face_ids = sorted(set(int(i) for i in group['ids']) - excluded)
        if not face_ids:
            continue
        missing = set(face_ids) - set(areas)
        if missing:
            raise ValueError('%s has %d scoped faces with unreadable area; refuse unbounded refinement' % (category,len(missing)))
        subtotal = 0.0
        overlapping=0
        for face_id in face_ids:
            old=effective_sizes.get(face_id)
            if old is not None:
                overlapping+=1
            if old is None or size < old:
                before=0.0 if old is None else areas[face_id]/(old*old)
                after=areas[face_id]/(size*size)
                subtotal+=after-before
                effective_sizes[face_id]=size
        if math.isnan(subtotal) or math.isinf(subtotal):
            raise ValueError('Invalid surface proxy')
        total += subtotal
        if subtotal <= 1.e-9:
            continue  # covered by an equally fine or finer existing control
        details.append({'category':category,'group_key':group.get('group_key'),
                        'diameter_band':group.get('diameter_band'),
                        'size_mm':size, 'face_count':len(face_ids),
                        'overlapping_faces':overlapping,
                        'incremental_surface_proxy':int(math.ceil(subtotal))})
    result = {'surface_proxy':int(math.ceil(total)),
              'limit':max_surface_proxy, 'sizing_groups':details,
              'unique_scoped_faces':len(effective_sizes),
              'meaning':'Surface area / size^2 budget veto, NOT volume mesh RAM prediction'}
    # Secondary, deliberately coarse guard for requested global sizing. The
    # volume / base_size^3 ratio is NOT a volume-element or memory prediction.
    # Never apply this guard if global meshing is operator-managed / unknown.
    if base_mm is not None:
        baseline=float(base_mm)
        if not 0.35 <= baseline <= 500 or math.isnan(baseline) or math.isinf(baseline):
            raise ValueError('Invalid global baseline size for volume budget')
        volume=0.0
        for body in state.get('bodies',[]):
            raw=float(body.get('volume',0))
            if raw<0 or math.isnan(raw) or math.isinf(raw):
                raise ValueError('Invalid fluid geometry volume for budget')
            volume+=raw*factor**3
        result['global_volume_proxy']=int(math.ceil(volume/(baseline**3)))
        result['volume_proxy_limit']=max_volume_proxy
        result['global_guard_applied']=bool(enforce_global)
        if enforce_global and result['global_volume_proxy']>max_volume_proxy:
            raise ValueError('32GB global volume budget warning: proxy %d > %d at %.3fmm; choose a coarser global size or review in Meshing. This is not a cell/RAM prediction.' %
                             (result['global_volume_proxy'],max_volume_proxy,baseline))
    if total > max_surface_proxy:
        raise ValueError('32GB surface refinement budget exceeded: proxy %d > %d after overrides (includes holes, gaps and ports). Reduce local refinement or disable a group.' %
                         (math.ceil(total),max_surface_proxy))
    return result



# v0.10.1: Model-agnostic FACE AREA inventory and groups.
# Area is the PRIMARY sorting feature. Surface family is a secondary separator.
# Geometry roles are never guessed from CAD face shape or proximity.
def _area_surface_family(raw):
    t=safe_text(raw or '').lower()
    if 'plan' in t: return 'Plane'
    if 'cylind' in t: return 'Cylinder'
    if any(x in t for x in ('spher','conic','cone','toroid')): return 'AnalyticCurve'
    if any(x in t for x in ('spline','nurbs','bsurf','freeform','surface')): return 'Freeform'
    return 'Other'


def classify_face_areas(state, base_mm, max_groups=4096, max_faces=90000):
    """Area-ranked, scale-relative geometry groups suitable for editable ACT names.

    Every physical face belongs to at most one auto group. Existing user named
    groups get priority, never renamed or re-scoped. Non-repeated large planar
    faces are REVIEW-only, preventing blanket baffle/outer-wall refinement.
    The bounded candidate limit is reported explicitly; this is not complete
    semantic CAD object recognition.
    """
    if not 0.5 <= float(base_mm) <= 500.0:
        raise ValueError('Invalid reference size')
    factor=unit_to_mm(state.get('geometry_unit',''))
    if factor is None or factor<=0:
        raise ValueError('Unknown CAD length unit; area classification requires explicit mm factor')
    faces=state.get('faces') or []
    if not 0<len(faces)<=max_faces:
        raise ValueError('Invalid/excessive face count for area grouping')
    catalog={}
    bad=[]
    for face in faces:
        try:
            fid=int(face['id'])
            area=float(face['area'])*factor*factor
            if fid in catalog or not (area>0.0) or math.isnan(area) or math.isinf(area):
                bad.append(fid)
                continue
            catalog[fid]={'id':fid,'area':area,'family':_area_surface_family(face.get('type')),
                          'body':safe_text(face.get('body','unknown'))}
        except (KeyError,TypeError,ValueError,OverflowError):
            bad.append(face.get('id','unreadable'))
    if not catalog:
        raise ValueError('All surface areas invalid; do not publish classifications')
    used=set()
    named=[]
    ignored=[]
    # Existing user-owned Named Selections are selectable as-is and NEVER
    # replaced or duplicated. inlet/outlet are mandatory boundaries, not local
    # sizing suggestions; preserve those independently.
    for ns in sorted(state.get('named',[]),key=lambda x:safe_text(x.get('name','')).lower()):
        name=safe_text(ns.get('name',''))
        lname=name.strip().lower()
        if not lname or lname.startswith('am_') or lname in ('inlet','outlet'):
            continue
        ids=set()
        for item in ns.get('ids',[]):
            try:
                i=int(item)
                if i in catalog:ids.add(i)
            except (TypeError,ValueError):pass
        if not ids:continue
        overlap=ids&used
        if overlap:
            ignored.append(name)
            # Never silently trim a user-owned named selection and claim it is
            # the same selection. Skip conflicting names and explain why.
            continue
        if len(ids)>20000:
            ignored.append(name)
            continue
        used.update(ids)
        arealist=sorted(catalog[i]['area'] for i in ids)
        named.append({'category':'named_existing','region_key':name,'ids':sorted(ids),
                      'mm':round(max(0.6,min(base_mm*0.5, math.sqrt(arealist[len(arealist)//2])/8.0)),3),
                      'confidence':'named','native_function':'uniform',
                      'area_mm2_min':round(arealist[0],3),'area_mm2_max':round(arealist[-1],3),
                      'surface_family':'UserSelection','reason':'Existing user-owned Named Selection; preserved unchanged'})
    # Inventory includes EVERY valid face, including already-named ones. User
    # names are protected at publication time, not hidden from the area audit.
    # ANSYS-native Worksheet Face/Size is the authority at publishing when its
    # range matches the exact intended scope. Grouping here is merely a proposal.
    ratio=1.22
    buckets={}
    for fid,face in catalog.items():
        area=face['area']
        band=int(math.floor(math.log(area)/math.log(ratio)))
        key=(face['family'],band,face['body'])
        buckets.setdefault(key,[]).append(fid)
    cutoff_area=(2.5*base_mm)**2
    groups=[]
    for (family,band,body),ids in buckets.items():
        ids.sort(key=lambda i:(catalog[i]['area'],i))
        areas=[catalog[i]['area'] for i in ids]
        count=len(ids)
        amin,amax=areas[0],areas[-1]
        repeat=count>=8
        small=amax<=cutoff_area
        category=('area_small' if small else 'area_repeated' if repeat else 'area_review')
        size=round(max(0.5,min(base_mm*0.55,math.sqrt(amin)/4.0)),3)
        # Large repeated surfaces (e.g. hundreds of wafer sides) should be
        # available for review/editing, even though they are not tiny holes.
        if category=='area_repeated':
            size=round(max(0.7,min(base_mm*0.55,math.sqrt(amin)/10.0)),3)
        # Keep stable group names independent of actual CAD face IDs. The
        # region key intentionally contains only class, area bin, and body.
        bandtag=('N%d'%-band if band<0 else 'P%d'%band)
        bodytag=''.join(ch if ch.isalnum() else '_' for ch in body)[:12]
        region='%s_A%s_B%s'%(family,bandtag,bodytag)
        groups.append({'category':category,'region_key':region,'ids':ids,
                       'mm':size,'confidence':('medium' if repeat else 'low'),
                       'native_function':'uniform',
                       'surface_family':family,
                       'area_mm2_min':round(amin,4),'area_mm2_max':round(amax,4),
                       'reason':('Area-ranked similar %s surfaces; %d faces, no CAD functional role inferred'%(family,count))})
    # Surface area is the PRIMARY sort key, and no category quota is allowed to
    # silently discard a family of holes. Large isolated planes remain review.
    groups.sort(key=lambda g:(g['area_mm2_min'],g['area_mm2_max'],g['surface_family'],g['region_key']))
    kept=groups[:max_groups]
    omitted=groups[max_groups:]
    # Limit only what is automatically ENABLED, not what can be seen and
    # inspected. Additional groups are opt-in individually in the UI.
    for g in kept:
        g['default_enabled']=False
        g['selection_method']='worksheet_size_if_exact_else_face_ids'
    # Rank suggested controls by evidence of repeated SMALL features, not
    # merely by the first 24 smallest area bands. In particular, repeated
    # cylindrical faces must not disappear behind incidental tiny CAD faces.
    eligible=[]
    for g in kept:
        n=len(g['ids'])
        is_small=(g['category']=='area_small' and n>=2)
        is_repeated=(g['category']=='area_repeated' and n>=8 and
                     g['area_mm2_max'] <= (base_mm*base_mm*100.0))
        if not (is_small or is_repeated):continue
        cyl=(g['surface_family']=='Cylinder')
        rank=(0 if (cyl and n>=4) else 1 if is_small else 2,
              -n,g['area_mm2_min'],g['region_key'])
        eligible.append((rank,g))
    defaults=0
    for _,g in sorted(eligible,key=lambda item:item[0])[:24]:
        g['default_enabled']=True
        defaults+=1
    chosen=set(g['region_key'] for g in kept)
    # Named user groups are preserved; limit only auto-generated candidates.
    output=named+kept
    seen=set()
    for g in output:
        key=g['category']+':'+g['region_key']
        if key in seen:
            raise RuntimeError('Non-unique area-group identity; review classifier')
        g['group_key']=key
        seen.add(key)
    evidence={'grouping_algorithm':'native_area_ranked_v2','geometry_unit':state.get('geometry_unit'),
              'unit_to_mm':factor,'face_count':len(faces),'valid_area_faces':len(catalog),
              'invalid_area_faces':len(bad),'named_user_groups':len(named),
              'named_conflict_skipped':ignored[:20],
              'candidate_groups_total':len(groups), 'candidate_groups_published':len(kept),
              'default_enabled_groups':defaults,
              'named_faces_in_area_inventory':len(used),
              'candidate_groups_omitted':len(omitted),
              'candidate_faces_omitted':sum(len(g['ids']) for g in omitted),
              'unfiltered_inventory_faces':len(catalog),
              'auto_selected_faces':sum(len(g['ids']) for g in kept),
              'cutoff_small_area_mm2':round(cutoff_area,2),
              'default_review_category':'area_review',
              'area_range_mm2':[round(min(g['area'] for g in catalog.values()),3),round(max(g['area'] for g in catalog.values()),3)],
              'warnings':[]}
    if omitted:
        evidence['warnings'].append('Candidate cap: %d groups / %d faces not exposed; no silent full-coverage claim'%(len(omitted),evidence['candidate_faces_omitted']))
    if bad:
        evidence['warnings'].append('%d faces had invalid/duplicate area and were skipped'%len(bad))
    if ignored:
        evidence['warnings'].append('Existing user Named Selections overlap or are too large; skipped for unchanged-scope protection')
    return output,evidence

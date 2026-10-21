#!/usr/bin/env python3
"""在 114 上通过既有管理员应用凭据注册本系统；不输出凭据。"""
import copy
import json
import os
import secrets
import urllib.parse
import urllib.request
from pathlib import Path

root = Path('/data/RevenueStatisticsSystem')
private = root / 'config'
private.mkdir(mode=0o700, exist_ok=True)
os.chmod(private, 0o700)
admin_id = Path('/etc/casdoor/vben-oidc-client-id').read_text().strip()
admin_secret = Path('/etc/casdoor/vben-oidc-client-secret').read_text().strip()

def api(path, body=None):
    query = urllib.parse.urlencode({'clientId': admin_id, 'clientSecret': admin_secret})
    url = 'http://127.0.0.1:8012' + path + ('&' if '?' in path else '?') + query
    raw = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(url, data=raw, headers={'Content-Type':'application/json'})
    with urllib.request.urlopen(req, timeout=15) as response:
        result = json.load(response)
    if result.get('status') != 'ok':
        raise RuntimeError('统一认证配置操作被拒绝: ' + result.get('msg', 'unknown'))
    return result.get('data')

apps = api('/api/get-applications?owner=admin') or []
existing = next((a for a in apps if a['name'] == 'app-revenuestatistics'), None)
if existing:
    if existing.get('organization') != 'sx' or existing.get('redirectUris') != ['http://cd.libeasy.cn:8027/api/auth/callback']:
        raise RuntimeError('同名应用已存在且配置不一致，未覆盖')
    app = existing
else:
    template = api('/api/get-application?id=admin/app-toushujianyi')
    if not template:
        raise RuntimeError('未发现可用的统一认证应用模板')
    app = copy.deepcopy(template)
    for key, value in list(app.items()):
        if isinstance(value, str) and key not in ('clientId','clientSecret'):
            app[key] = value.replace(':8018', ':8027').replace('投诉建议', '营收统计')
    app.update(name='app-revenuestatistics', displayName='营收统计系统', description='寿县九类经营业态营收统计与接口对接', clientId=secrets.token_hex(16), clientSecret=secrets.token_urlsafe(48), redirectUris=['http://cd.libeasy.cn:8027/api/auth/callback'], grantTypes=['authorization_code'], tokenFormat='JWT', expireInHours=8, enableSignUp=False, homepageUrl='http://cd.libeasy.cn:8027/', logo='http://cd.libeasy.cn:8027/revenue-logo.png')
    api('/api/add-application', app)
for field, name in [('clientId','oidc-client-id'), ('clientSecret','oidc-client-secret')]:
    path = private / name
    path.write_text(app[field])
    os.chmod(path, 0o600)
permissions = api('/api/get-permissions?owner=sx') or []
original = next((p for p in permissions if p['name'] == 'SuperAdmin' and p.get('isEnabled') and p.get('effect') == 'Allow' and p.get('state') == 'Approved'), None)
if not original:
    raise RuntimeError('未发现有效的原明确超级许可，未自动按用户名授权')
roles = api('/api/get-roles?owner=sx') or []
for name, display in [('RevenueStatisticsSystem.Access','营收统计系统访问'), ('RevenueStatisticsSystem.SuperAdmin','营收统计系统超级用户')]:
    if any(p['name'] == name for p in permissions):
        continue
    permission = copy.deepcopy(original)
    permission.update(name=name, displayName=display, resources=['app-revenuestatistics'], model='', adapter='', resourceType='Application', actions=['Read'], startTime=None, endTime=None)
    if name.endswith('.Access'):
        permission['roles'] = list(dict.fromkeys((permission.get('roles') or []) + [r['owner']+'/'+r['name'] for r in roles if r['name']=='vben-user' and r.get('isEnabled')]))
    api('/api/add-permission', permission)
check = api('/api/get-application?id=admin/app-revenuestatistics')
new_permissions = api('/api/get-permissions?owner=sx')
print(json.dumps({'application':check['name'], 'organization':check['organization'], 'redirectUris':check['redirectUris'], 'permissions':[p['name'] for p in new_permissions if p['name'].startswith('RevenueStatisticsSystem.')], 'credentials':'stored in restricted server config'}, ensure_ascii=False))




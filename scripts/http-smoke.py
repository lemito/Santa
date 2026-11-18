#!/usr/bin/env python3
"""Real HTTP regression checks. Uses unique accounts; no database cleanup."""
import http.cookiejar
import json
import re
import urllib.request
import urllib.parse
import urllib.error
import uuid
import os

BASE = os.environ.get('SANTA_TEST_URL','http://127.0.0.1:5080')
class Client:
    def __init__(self):
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    def request(self,path,body=None,method=None,csrf=True,form=False):
        headers={}
        if body is not None:
            if form:
                body=urllib.parse.urlencode(body).encode();headers['Content-Type']='application/x-www-form-urlencoded'
            else:
                body=json.dumps(body).encode();headers['Content-Type']='application/json'
                if csrf:
                    _,session=self.request('/api/session')
                    session=json.loads(session)
                    headers['X-CSRF-TOKEN']=session.get('csrf') or session['csrfToken']
        request=urllib.request.Request(BASE+path,body,headers,method=method)
        try:
            with self.opener.open(request) as response: return response.status,response.read().decode()
        except urllib.error.HTTPError as e: return e.code,e.read().decode()
    def register(self,email,name):
        status,page=self.request('/register');assert status==200
        match=re.search(r'name="__RequestVerificationToken" value="([^"]+)"',page)
        body={'email':email,'name':name,'displayName':name,'password':'Test-Gift-Password42!'}
        if match:body['__RequestVerificationToken']=match[1]
        status,page=self.request('/account/register',body,form=True)
        assert status==200 and 'Новая игра' in page,(status,page[:500])

clients=[Client() for _ in range(4)]
suffix=uuid.uuid4().hex[:10]
emails=[f'http-{suffix}-{i}@example.test' for i in range(4)]
for i,c in enumerate(clients):c.register(emails[i],f'Guest {i}')
owner,a,b,outsider=clients
anon=Client()
assert anon.request('/api/games/')[0]==401
assert owner.request('/api/games/',{'title':'missing csrf'},csrf=False)[0] in (400,403)
status,data=owner.request('/api/games/',{'title':'HTTP test'})
assert status in (200,201),(status,data)
game=json.loads(data)['id'];path='/api/games/'+game
status,data=owner.request(path+'/configuration',{'version':1,'emails':emails[:3],'exclusions':[]},method='PUT');assert status in (200,204),(status,data)
assert outsider.request(path)[0]==404
assert a.request(path+'/configuration')[0]==403
assert a.request(path+'/publish',{'version':2,'requestKey':str(uuid.uuid4())})[0]==403
messages={}
for i,c in enumerate(clients[:3]):
    marker=f'wish-{suffix}-{i}'
    markdown=f'## Мои пожелания\n\n- **{marker}**\n\n![Подарок](https://example.test/{marker}.png)\n\n![Опасно](data:text/html;base64,PHNjcmlwdD4=)\n\n<script>alert(1)</script>\n\n[опасно](javascript:alert(1))'
    messages[emails[i]]=marker
    status,data=c.request(path+'/my-message',{'message':markdown},method='PUT');assert status==204,(status,data)
    status,data=c.request(path+'/my-message');assert status==200,(status,data)
    value=json.loads(data);assert value['markdown']==markdown and '<strong>'+marker+'</strong>' in value['html']
    assert '<img' in value['html'].lower() and f'https://example.test/{marker}.png' in value['html']
    assert '<script' not in value['html'].lower() and 'javascript:' not in value['html'].lower() and 'data:' not in value['html'].lower()
assert outsider.request(path+'/my-message',{'message':'no access'},method='PUT')[0]==403
assert a.request(path+'/my-message',{'message':'x'*5001},method='PUT')[0] in (400,422)
assert a.request(path+'/my-recipient-message',{'message':'not drawn'},method='PUT')[0]==409
key=str(uuid.uuid4())
status,data=owner.request(path+'/publish',{'version':2,'requestKey':key});assert status==200,(status,data)
assert owner.request(path+'/publish',{'version':2,'requestKey':key})[0]==200
assert owner.request(path+'/publish',{'version':2,'requestKey':str(uuid.uuid4())})[0]==409
assert owner.request(path+'/configuration',{'version':2,'emails':emails,'exclusions':[]},method='PUT')[0]==409
recipients=[]
from_santa={}
for i,c in enumerate(clients[:3]):
    status,data=c.request(path+'/my-assignment');assert status==200,(status,data)
    value=json.loads(data);assert set(value)=={'name','email','messageHtml'} and value['email']!=emails[i]
    assert '<strong>'+messages[value['email']]+'</strong>' in value['messageHtml']
    assert '<script' not in value['messageHtml'].lower() and 'javascript:' not in value['messageHtml'].lower()
    recipients.append(value['email'])
    marker=f'santa-{suffix}-{i}'
    reply=f'## От Санты\n\n**{marker}**\n\n![Ёлка](https://example.test/{marker}.png)\n\n![Опасно](javascript:alert(1))'
    status,data=c.request(path+'/my-recipient-message',{'message':reply},method='PUT');assert status==204,(status,data)
    status,data=c.request(path+'/my-recipient-message');assert status==200,(status,data)
    outgoing=json.loads(data);assert outgoing['markdown']==reply and '<strong>'+marker+'</strong>' in outgoing['html']
    assert 'javascript:' not in outgoing['html'].lower()
    from_santa[value['email']]=marker
    status,page=c.request('/games/'+game)
    assert status==200 and value['email'] in page and 'id="giver-role"' in page and 'id="receiver-role"' in page
    assert 'class="games-nav-link"' in page and 'class="back-link"' not in page
    assert 'recipient-preview' in page and 'santa-preview' in page and 'Редактировать' in page
    # Published HTML contains the one recipient; no full configuration or result.
    for other in emails[:3]:
        if other!=value['email']:assert other not in page
assert set(recipients)==set(emails[:3])
for i,c in enumerate(clients[:3]):
    status,data=c.request(path+'/message-from-santa');assert status==200,(status,data)
    incoming=json.loads(data);marker=from_santa[emails[i]]
    assert '<strong>'+marker+'</strong>' in incoming['html'] and f'https://example.test/{marker}.png' in incoming['html']
    assert 'javascript:' not in incoming['html'].lower()
    status,page=c.request('/games/'+game)
    assert status==200 and 'От вашего Санты' in page and marker in page
assert outsider.request(path+'/my-assignment')[0]==404
assert outsider.request(path+'/my-recipient-message',{'message':'no access'},method='PUT')[0]==403
assert outsider.request(path+'/message-from-santa')[0]==404
# A client-supplied giver doesn't override the authenticated identity.
status,data=a.request(path+'/my-assignment?giverId='+uuid.uuid4().hex)
assert status==200 and json.loads(data)['email']==recipients[1]
assert owner.request(path+'/close',{})[0] in (200,204)
assert a.request(path+'/my-assignment')[0]==404
assert a.request(path+'/my-message',{'message':'too late'},method='PUT')[0]==409
assert a.request(path+'/my-recipient-message',{'message':'too late'},method='PUT')[0]==409
assert a.request(path+'/message-from-santa')[0]==404
# Login/logout and form CSRF.
status,page=owner.request('/')
match=re.search(r'name="(__RequestVerificationToken|csrfToken)" value="([^"]+)"',page)
assert match
assert owner.request('/account/logout',{match[1]:match[2]},form=True)[0]==200
assert owner.request('/api/games/')[0]==401
status,page=owner.request('/login');match=re.search(r'name="__RequestVerificationToken" value="([^"]+)"',page)
body={'email':emails[0],'password':'Test-Gift-Password42!'}
if match:body['__RequestVerificationToken']=match[1]
status,page=owner.request('/account/login',body,form=True)
assert status==200 and 'Новая игра' in page
print('PASS: registration, CSRF, game flow, two-way anonymous messages, safe Markdown images, permissions, retries and close')

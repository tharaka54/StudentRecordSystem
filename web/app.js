'use strict';

const $ = id => document.getElementById(id);
const state = {user:null,csrf:'',route:'',sequence:0,lists:{},lookups:null,student:null};
const paths = {
 dashboard:'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
 students:'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2 M16 3a4 4 0 0 1 0 8 M22 21v-2a4 4 0 0 0-3-3.87 M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
 courses:'M4 19.5A2.5 2.5 0 0 1 6.5 17H20 M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2',
 units:'M4 4h6v6H4z M14 4h6v6h-6z M4 14h6v6H4z M14 14h6v6h-6z',
 enrolments:'M8 2v4 M16 2v4 M3 10h18 M5 4h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2 M8 15l3 3 5-5',
 results:'M9 3H5a2 2 0 0 0-2 2v15a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V5a2 2 0 0 0-2-2h-4 M9 2h6v4H9z M8 12h8 M8 17h5',
 users:'M12 22s8-4 8-11V5l-8-3-8 3v6c0 7 8 11 8 11 M9 12l2 2 4-4',
 profile:'M20 21v-2a7 7 0 0 0-14 0v2 M17 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
 summary:'M4 3h16v18H4z M8 7h8 M8 11h8 M8 15h3 M14 15h2',
 password:'M5 10h14v11H5z M8 10V6a4 4 0 0 1 8 0v4 M12 14v3',
 search:'M21 21l-5-5 M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0',
 plus:'M12 5v14 M5 12h14', refresh:'M20 7v5h-5 M4 17v-5h5 M6 7a7 7 0 0 1 12-2l2 2 M18 17a7 7 0 0 1-12 2l-2-2',
 arrow:'M5 12h14 M13 6l6 6-6 6', back:'M19 12H5 M11 6l-6 6 6 6', close:'M6 6l12 12 M6 18L18 6', menu:'M3 6h18 M3 12h18 M3 18h18', logout:'M9 21H4V3h5 M13 8l5 4-5 4 M8 12h10'
};
function icon(name) { const s=document.createElementNS('http://www.w3.org/2000/svg','svg');s.setAttribute('viewBox','0 0 24 24');s.setAttribute('fill','none');s.setAttribute('stroke','currentColor');s.setAttribute('stroke-width','1.6');s.setAttribute('stroke-linecap','round');s.setAttribute('stroke-linejoin','round');s.setAttribute('aria-hidden','true');const p=document.createElementNS(s.namespaceURI,'path');p.setAttribute('d',paths[name]||paths.units);s.append(p);return s; }
function h(tag,attrs={},...children) { const n=document.createElement(tag);for(const [k,v] of Object.entries(attrs)){if(v===undefined||v===null||v===false)continue;if(k==='class')n.className=v;else if(k.startsWith('on'))n.addEventListener(k.slice(2),v);else if(k==='text')n.textContent=v;else if(k in n)n[k]=v;else n.setAttribute(k,String(v));}for(const child of children.flat(Infinity)){if(child===null||child===undefined||child===false)continue;n.append(child instanceof Node?child:document.createTextNode(String(child)));}return n; }
const text = v => v===null||v===undefined||v===''?'—':String(v);
const initials = name => String(name).trim().split(/\s+/).map(p=>p[0]).slice(0,2).join('').toUpperCase();
const formatMark = v => v===null||v===undefined?'—':Number(v).toFixed(2);
function btn(label,action,kind='',symbol) {return h('button',{class:'btn '+kind,type:'button',onclick:action},symbol&&icon(symbol),label);}
function link(label,route,cls='') {return h('a',{href:'#'+route,class:cls},label);}
function notify(message,error=false) {clearTimeout(notify.timer);$('notification').textContent=message;$('notification').className='notification'+(error?' error':'');$('notification').hidden=false;notify.timer=setTimeout(()=>$('notification').hidden=true,5000);}
function setMessage(node,message,success=false) {node.textContent=message;node.className='form-message'+(success?' success':'');node.hidden=!message;}

async function api(path,options={}) {
 const headers={...options.headers};if(options.body!==undefined){headers['Content-Type']='application/json';options.body=JSON.stringify(options.body);}if(state.csrf)headers['X-CSRF-Token']=state.csrf;
 let response;try{response=await fetch('/api/'+path,{credentials:'same-origin',...options,headers});}catch{throw Error('Unable to connect. Please try again.');}
 const data=await response.json().catch(()=>({error:'The request could not be completed.'}));
 if(!response.ok){if(response.status===401&&path!=='login')showLogin('Your session has ended. Please sign in again.');throw Error(data.error||'The request could not be completed.');}return data;
}
function showLogin(message='',success=false) {state.user=null;state.csrf='';state.lookups=null;state.student=null;state.lists={};state.sequence++;$('editor').close();$('confirm-dialog').close();$('content').replaceChildren();$('app-shell').hidden=true;$('login-screen').hidden=false;$('password').value='';setMessage($('login-message'),message,success);closeNavigation();}
function openApp(auth) {state.user=auth.user;state.csrf=auth.csrf_token;$('login-screen').hidden=true;$('app-shell').hidden=false;$('sidebar-name').textContent=auth.user.FullName;$('sidebar-role').textContent=auth.user.Role;$('avatar').textContent=initials(auth.user.FullName);$('role-label').textContent=auth.user.Role;$('portal-name').textContent=auth.user.Role==='Admin'?'Academic Registry':'Student Portal';$('today').textContent=new Intl.DateTimeFormat('en-GB',{day:'numeric',month:'short',year:'numeric'}).format(new Date());makeNavigation();renderRoute();}
function makeNavigation(){const admin=state.user.Role==='Admin';const items=admin?[['dashboard','Dashboard'],['students','Students'],['courses','Courses'],['units','Units'],['enrolments','Enrolments'],['results','Results'],['users','Student accounts'],['password','Account security']]:[['dashboard','Dashboard'],['profile','My profile'],['enrolments','My enrolments'],['results','My results'],['summary','Academic summary'],['password','Change password']];$('navigation').replaceChildren(...items.map(([id,label])=>h('a',{href:'#'+id,class:'nav-link','data-route':id},icon(id),label)));}
function closeNavigation(){$('sidebar').classList.remove('open');$('nav-backdrop').hidden=true;$('menu-toggle').setAttribute('aria-expanded','false');}
function navigate(route){if(location.hash==='#'+route)renderRoute();else location.hash=route;}
function heading(title,description,...actions){return h('div',{class:'page-head'},h('div',{},h('h1',{},title),description&&h('p',{},description)),actions.length>0&&h('div',{class:'head-actions'},actions));}
function panel(title,body,action){return h('section',{class:'panel'},h('div',{class:'section-title'},h('h2',{},title),action),body);}
function metric(label,value,note,symbol){return h('div',{class:'stat'},h('div',{class:'stat-label'},label,icon(symbol)),h('strong',{class:'stat-value'},value),h('span',{class:'stat-note'},note));}
function badge(value){const colors={Active:'green',Enrolled:'blue',Completed:'green',Withdrawn:'grey',Inactive:'grey',Disabled:'grey',HD:'green',D:'blue',C:'blue',P:'amber',F:'red'};return h('span',{class:'badge '+(colors[value]||'grey')},text(value));}
function empty(title='No records found',description='There are no records to display.'){return h('div',{class:'empty'},h('strong',{},title),description);}
function table(columns,rows,renderer){if(!rows.length)return empty();return h('div',{class:'table-wrap'},h('table',{},h('thead',{},h('tr',{},columns.map(c=>h('th',{scope:'col'},c)))),h('tbody',{},rows.map(row=>h('tr',{},renderer(row).map(v=>h('td',{},v)))))));}
function person(row){const name=row.student_name||[row.first_name,row.last_name].filter(Boolean).join(' ');return h('div',{class:'person'},h('span',{class:'person-avatar'},initials(name)),h('div',{},h('span',{class:'primary-text'},name),h('span',{class:'secondary-text'},row.student_number||row.email)));}
function errorPage(error){return h('section',{class:'panel panel-pad'},h('h2',{},'Unable to load this page'),h('p',{class:'muted'},error.message),btn('Try again',()=>renderRoute(),'','refresh'));}

async function renderRoute(){
 if(!state.user)return;
 const route=decodeURIComponent(location.hash.slice(1)||'dashboard');state.route=route;const seq=++state.sequence;closeNavigation();$('editor').close();$('confirm-dialog').close();
 const parts=route.split('/'),base=parts[0];document.querySelectorAll('#navigation a').forEach(a=>{const active=a.dataset.route===base;a.classList.toggle('active',active);if(active)a.setAttribute('aria-current','page');else a.removeAttribute('aria-current');});
 $('breadcrumb').textContent=base==='dashboard'?'Dashboard':base==='password'?'Account security':(RESOURCES[base]?.title||{profile:'My profile',summary:'Academic summary'}[base]||'Records');
 $('content').replaceChildren(h('p',{class:'loading',role:'status'},'Loading records…'));
 try{
  let view;
  if(base==='password')view=passwordPage();
  else if(state.user.Role==='Student'){
   if(!['dashboard','profile','enrolments','results','summary'].includes(base)||parts.length>1)throw Error('This page is not available for your account.');
   const data=await api('student/overview');state.student=data;
   view=base==='dashboard'?studentDashboard(data):base==='profile'?profilePage(data):base==='summary'?summaryPage(data):studentRecords(base,data);
  } else if(base==='dashboard'){view=adminDashboard(await api('dashboard'));}
  else if(base==='students'&&parts[1]==='new'){view=await studentEditor(null);}
  else if(base==='students'&&/^\d+$/.test(parts[1]||'')){view=parts[2]==='edit'?await studentEditor(await api('students/'+parts[1])):summaryPage(await api('students/'+parts[1]+'/summary'),true);}
  else if(RESOURCES[base]&&parts.length===1){view=await resourcePage(base);}
  else throw Error('Page not found. Choose a page from the navigation.');
  if(seq!==state.sequence||!state.user)return;$('content').replaceChildren(view);$('content').focus({preventScroll:true});
 }catch(error){if(seq===state.sequence&&state.user)$('content').replaceChildren(errorPage(error));}
}

function adminDashboard(data){
 const c=data.counts;
 const recent=table(['Student','Course','Status',''],data.recent_students,r=>[person(r),h('span',{class:'code'},r.course_name),badge(r.status),link('View record','students/'+r.student_id)]);
 const quick=h('div',{class:'panel-pad'},[['Add a student','students/new'],['Manage enrolments','enrolments'],['Enter results','results'],['Manage student accounts','users']].map(([label,route])=>h('a',{class:'quick-action',href:'#'+route},label,icon('arrow'))));
 const overview=h('div',{class:'panel-pad'},[['Enrolments',c.enrolments],['Results recorded',c.results],['Weighted average mark',formatMark(data.overall_wam)]].map(([label,value])=>h('div',{class:'info-row'},h('span',{class:'muted'},label),h('strong',{},value))));
 return h('div',{},heading('Dashboard','An overview of student records and academic activity.',link('Add student','students/new','btn primary')),h('div',{class:'stats'},metric('Total students',c.students,'Registered student records','students'),metric('Courses',c.courses,'Courses in the catalogue','courses'),metric('Units',c.units,'Units available for enrolment','units'),metric('Results recorded',c.results,'Graded unit enrolments','results')),h('div',{class:'dashboard-grid'},panel('Recently registered students',recent,link('View all students','students')),h('div',{class:'stack'},panel('Academic overview',overview),panel('Quick actions',quick))));
}
function studentDashboard(data){const s=data.student;const current=data.units.filter(u=>u.status==='Enrolled');return h('div',{},heading('My dashboard','Welcome, '+s.first_name+'. Here is your academic overview.'),h('div',{class:'stats'},metric('Weighted average mark',formatMark(data.wam),'Based on completed, graded units','results'),metric('Enrolled units',data.enrolled_units,'Current unit registrations','enrolments'),metric('Graded units',data.graded_units,'Units with published results','units'),metric('Credits earned',data.earned_credits,'Credits from passed units','courses')),h('div',{class:'dashboard-grid'},panel('Current enrolments',unitTable(current),link('View all enrolments','enrolments')),h('div',{class:'stack'},panel('My course',h('div',{class:'panel-pad'},h('span',{class:'code'},s.course_code),h('h3',{class:'course-heading'},s.course_name),h('div',{class:'info-row'},h('span',{class:'muted'},'Student number'),h('strong',{},s.student_number)),h('div',{class:'info-row'},h('span',{class:'muted'},'Registration status'),badge(s.status)),h('div',{class:'info-row'},h('span',{class:'muted'},'Duration'),s.duration_years+' years'))),panel('Academic services',h('div',{class:'panel-pad'},[['View my results','results'],['Academic summary','summary'],['My profile','profile']].map(([label,route])=>h('a',{class:'quick-action',href:'#'+route},label,icon('arrow'))))))));}
function unitTable(rows,withResults=false){return table(withResults?['Unit','Academic period','Credits','Mark','Grade']:['Unit','Academic period','Credits','Status'],rows,u=>[h('div',{},h('span',{class:'primary-text'},u.unit_code),h('span',{class:'secondary-text'},u.unit_name)),u.academic_year+' · Semester '+u.semester,u.credit_points,...(withResults?[h('span',{class:'grade'},formatMark(u.mark)),badge(u.grade)]:[badge(u.status)])]);}
function profilePage(data) {
 const s=data.student;
 const fields=[['Student number',s.student_number],['Registration status',s.status],['Email address',s.email],['Phone number',s.phone],['Date of birth',s.date_of_birth],['Address',s.address],['Course',s.course_name],['Course code',s.course_code]];
 const details=h('dl',{class:'profile-grid'});
 for(const [label,value] of fields) details.append(h('div',{},h('dt',{},label),h('dd',{},text(value))));
 const identity=h('div',{class:'profile-head'},h('span',{class:'profile-initials'},initials(s.first_name+' '+s.last_name)),h('div',{},h('h2',{},s.first_name+' '+s.last_name),h('p',{},s.student_number+' · '+s.course_code)));
 return h('div',{},heading('My profile','Your personal details and course registration.'),h('section',{class:'panel form-panel'},identity,details));
}
function summaryPage(data,admin=false){const s=data.student;const groups=new Map();for(const u of data.units){const key=u.academic_year+' · Semester '+u.semester;if(!groups.has(key))groups.set(key,[]);groups.get(key).push(u);}const history=h('div',{});for(const [term,units] of groups)history.append(h('div',{class:'term-heading'},term),table(['Unit','Credits','Status','Mark','Grade'],units,u=>[h('div',{},h('span',{class:'primary-text'},u.unit_code),h('span',{class:'secondary-text'},u.unit_name)),u.credit_points,badge(u.status),formatMark(u.mark),badge(u.grade)]));if(!groups.size)history.append(empty('No academic history','No unit enrolments have been recorded.'));return h('div',{},heading(admin?'Student academic summary':'Academic summary',s.first_name+' '+s.last_name+' · '+s.student_number,...(admin?[link('Back to students','students','btn'),link('Edit student','students/'+s.student_id+'/edit','btn primary')]:[])),h('div',{class:'stats'},metric('Weighted average mark',formatMark(data.wam),'Completed, graded units','results'),metric('Graded units',data.graded_units,'Including all published grades','units'),metric('Credits earned',data.earned_credits,'Passed unit credits','courses'),metric('Enrolled units',data.enrolled_units,'Current registrations','enrolments')),h('section',{class:'panel'},h('div',{class:'summary-course'},h('div',{},h('strong',{},s.course_name),h('p',{},s.course_code+' · '+s.duration_years+' years')),badge(s.status)),h('div',{class:'section-title'},h('h2',{},'Academic history')),history),h('p',{class:'muted',style:undefined},'Weighted average mark includes completed units with recorded marks, weighted by credit points. Ungraded and withdrawn units are excluded.'));}

function studentRecords(resource,data){const results=resource==='results',rows=results?data.units.filter(u=>u.mark!==null):data.units;const wrap=h('div',{});const search=h('input',{type:'search',placeholder:'Search unit code or name','aria-label':'Search units',maxlength:100});const filter=h('select',{'aria-label':results?'Filter by grade':'Filter by status'},h('option',{value:''},results?'All grades':'All statuses'),(results?['HD','D','C','P','F']:['Enrolled','Completed','Withdrawn']).map(v=>h('option',{value:v},v)));function draw(){const q=search.value.toLowerCase();wrap.replaceChildren(unitTable(rows.filter(u=>(u.unit_code+' '+u.unit_name).toLowerCase().includes(q)&&(!filter.value||(results?u.grade:u.status)===filter.value)),results));}search.addEventListener('input',draw);filter.addEventListener('change',draw);draw();return h('div',{},heading(results?'My results':'My enrolments',results?'Published marks and grades for your enrolled units.':'Your unit registrations across academic periods.'),h('section',{class:'panel'},h('div',{class:'toolbar'},h('div',{class:'filter-group'},h('div',{class:'search-wrap'},icon('search'),search),filter)),wrap));}

async function lookups(force=false){if(!state.lookups||force)state.lookups=await api('lookups');return state.lookups;}
function cell(row,key){if(key==='student_name')return person(row);if(key==='term')return row.academic_year+' · Semester '+row.semester;if(key==='status'||key==='grade')return badge(row[key]);if(key==='is_active')return badge(row.is_active?'Active':'Disabled');if(key==='mark')return h('span',{class:'grade'},formatMark(row.mark));if(key==='duration_years')return row[key]+' years';if(key==='unit_code')return h('div',{},h('span',{class:'primary-text'},row.unit_code),row.unit_name&&h('span',{class:'secondary-text'},row.unit_name));return h('span',{class:key.endsWith('_code')||key==='student_number'?'code':''},text(row[key]));}
async function resourcePage(resource){
 const def=RESOURCES[resource],settings=state.lists[resource]||(state.lists[resource]={q:'',page:1,page_size:10,filter:''});const view=h('div',{}),body=h('div',{}),pager=h('div',{class:'pagination'}),count=h('span',{class:'record-count'});let requestNumber=0;
 const search=h('input',{type:'search',value:settings.q,placeholder:def.search,'aria-label':def.search,maxLength:100});let filter,filterKey;
 if(resource==='students'||resource==='enrolments'){filterKey='status';filter=h('select',{'aria-label':'Filter by status'},h('option',{value:''},'All statuses'),(resource==='students'?['Active','Inactive']:['Enrolled','Completed','Withdrawn']).map(v=>h('option',{value:v},v)));}
 else if(resource==='results'){filterKey='grade';filter=h('select',{'aria-label':'Filter by grade'},h('option',{value:''},'All grades'),['HD','D','C','P','F'].map(v=>h('option',{value:v},v)));}
 else if(resource==='users'){filterKey='is_active';filter=h('select',{'aria-label':'Filter by account status'},h('option',{value:''},'All accounts'),h('option',{value:'1'},'Active'),h('option',{value:'0'},'Disabled'));}
 else if(resource==='units'){filterKey='course_id';const options=await lookups();filter=h('select',{'aria-label':'Filter by course'},h('option',{value:''},'All courses'),options.courses.map(c=>h('option',{value:c.course_id},c.course_code)));}
 if(filter)filter.value=settings.filter;
 async function load(){const n=++requestNumber;const qs=new URLSearchParams({q:settings.q,page:settings.page,page_size:settings.page_size});if(resource==='users')qs.set('role','Student');if(filter&&settings.filter)qs.set(filterKey,settings.filter);try{const data=await api(resource+'?'+qs);if(n!==requestNumber||!state.user)return;if(data.items.length===0&&data.total>0&&settings.page>1){settings.page=Math.max(1,Math.ceil(data.total/settings.page_size));return load();}body.replaceChildren(data.items.length?table([...def.columns.map(c=>c[1]),'Actions'],data.items,row=>[...def.columns.map(([key])=>cell(row,key)),h('div',{class:'table-actions'},resource==='students'&&link('View','students/'+row.student_id,'text-button'),h('button',{type:'button',class:'text-button',onclick:()=>resource==='students'?navigate('students/'+row.student_id+'/edit'):openEditor(resource,row,load)},'Edit'),h('button',{type:'button',class:'text-button danger-text',onclick:()=>confirmDelete(resource,row,load)},'Delete'))]):empty(settings.q||settings.filter?'No matching records':'No '+def.title.toLowerCase()+' yet',settings.q||settings.filter?'Try changing your search or filter.':'Select Add '+def.singular+' to create a record.'));count.textContent=data.total+' '+(data.total===1?'record':'records');const pages=Math.max(1,Math.ceil(data.total/settings.page_size));const prev=btn('Previous',()=>{settings.page--;load();},'small'),next=btn('Next',()=>{settings.page++;load();},'small');prev.disabled=settings.page<=1;next.disabled=settings.page>=pages;pager.replaceChildren(h('span',{},data.total?((settings.page-1)*settings.page_size+1)+'–'+Math.min(settings.page*settings.page_size,data.total)+' of '+data.total+' records':'0 records'),h('div',{class:'pagination-controls'},prev,h('span',{class:'page-label'},'Page '+settings.page+' of '+pages),next));}catch(error){if(n===requestNumber)body.replaceChildren(errorPage(error));}}
 let timer;search.addEventListener('input',()=>{settings.q=search.value;settings.page=1;clearTimeout(timer);timer=setTimeout(load,220);});if(filter)filter.addEventListener('change',()=>{settings.filter=filter.value;settings.page=1;load();});
 view.append(heading(def.title,def.description,btn('Refresh',()=>load(),'','refresh'),btn('Add '+def.singular,()=>resource==='students'?navigate('students/new'):openEditor(resource,null,load),'primary','plus')),h('section',{class:'panel'},h('div',{class:'toolbar'},h('div',{class:'filter-group'},h('div',{class:'search-wrap'},icon('search'),search),filter),count),body,pager));await load();return view;
}

async function studentEditor(existing){const back=existing?'students/'+existing.student_id:'students';return h('div',{},heading(existing?'Edit student':'Add student',existing?'Update student details and course registration.':'Create a student record and assign a course.',link('Back',back,'btn')),h('section',{class:'panel form-panel'},await recordForm('students',existing,()=>navigate(back),()=>navigate('students'))));}
async function openEditor(resource,row,onSaved){try{$('editor-title').textContent=(row?'Edit ':'Add ')+RESOURCES[resource].singular;$('editor-body').replaceChildren(await recordForm(resource,row,async()=>{$('editor').close();await onSaved();},()=>$('editor').close()));$('editor').showModal();}catch(error){notify(error.message,true);}}
async function recordForm(resource,existing,onSaved,onCancel){
 const def=RESOURCES[resource],options=await lookups(true),form=h('form',{class:'record-form'}),grid=h('div',{class:'form-grid'}),error=h('p',{class:'form-message',role:'alert',hidden:true}),inputs={};
 if(resource==='results')form.append(h('p',{class:'field-note'},'The grade is calculated automatically when a mark is saved.'));
 for(const [name,label,type,required,limit] of def.fields){const field=h('div',{class:'field'+(type==='textarea'||type==='enrolments'?' full-width':'')});const id='record-'+name;let input;
  if(['options','courses','students','units','enrolments','accountStudents'].includes(type)){
   input=h('select',{id,name,required});
   if(type==='options'){input.append(...limit.map(v=>h('option',{value:v},v)));}
   else {input.append(h('option',{value:''},'Select '+(type==='accountStudents'?'student':label.toLowerCase())));let rows=options[type==='accountStudents'?'students':type]||[];if(type==='accountStudents')rows=rows.filter(r=>!r.user_id||r.student_id===existing?.student_id);if(type==='enrolments')rows=rows.filter(r=>r.status!=='Withdrawn'&&(!r.result_id||r.enrolment_id===existing?.enrolment_id));for(const r of rows){const pk={courses:'course_id',students:'student_id',accountStudents:'student_id',units:'unit_id',enrolments:'enrolment_id'}[type];const label=type==='courses'?r.course_code+' · '+r.course_name:type==='units'?r.unit_code+' · '+r.unit_name:type==='enrolments'?r.student_number+' · '+r.first_name+' '+r.last_name+' · '+r.unit_code+' · '+r.academic_year+' S'+r.semester:r.student_number+' · '+r.first_name+' '+r.last_name;input.append(h('option',{value:r[pk]},label));}}
  }else if(type==='textarea')input=h('textarea',{id,name,maxLength:limit,required});
  else {input=h('input',{id,name,type:type==='decimal'?'number':type,required:required||(type==='password'&&!existing)});if(type==='number'||type==='decimal'){input.min=limit[0];input.max=limit[1];input.step=type==='decimal'?'0.01':'1';}else if(limit)input.maxLength=limit;if(type==='password'){input.minLength=10;input.autocomplete='new-password';if(existing)input.placeholder='Leave blank to keep the current password';}if(name==='date_of_birth')input.max=new Date().toISOString().slice(0,10);}
  let value=existing?.[name];if(name==='is_active')value=existing?(existing.is_active?'Active':'Disabled'):'Active';if(name==='password')value='';if(value===undefined){if(name==='academic_year')value=new Date().getFullYear();else if(name==='semester')value=2;else if(name==='enrolment_date')value=new Date().toISOString().slice(0,10);else if(name==='credit_points')value=15;else if(name==='duration_years')value=3;else if(name==='status')value=resource==='students'?'Active':'Enrolled';}if(value!==undefined&&value!==null)input.value=value;
  if(existing&&((resource==='users'&&name==='student_id')||(resource==='results'&&name==='enrolment_id')))input.disabled=true;
  inputs[name]=input;field.append(h('label',{htmlFor:id},label+(required?' *':'')),input);if(type==='password')field.append(h('small',{},existing?'Enter a new password only when resetting this account.':'Use at least 10 characters.'));grid.append(field);
 }
 if(resource==='enrolments'){
  const unitHelp=h('small',{id:'unit-help'});
  inputs.unit_id.setAttribute('aria-describedby','unit-help');
  inputs.unit_id.parentElement.append(unitHelp);
  const updateUnits=()=>{
   const student=options.students.find(s=>s.student_id===Number(inputs.student_id.value));
   const available=options.units.filter(u=>u.course_id===student?.course_id);
   const selected=Number(inputs.unit_id.value);
   const placeholder=!student?'Select a student first':available.length?'Select unit':'No units assigned to this course';
   // replaceChildren needs individual nodes, not an array converted into text.
   inputs.unit_id.replaceChildren(h('option',{value:''},placeholder),...available.map(u=>h('option',{value:u.unit_id},u.unit_code+' · '+u.unit_name)));
   inputs.unit_id.disabled=!student||available.length===0;
   inputs.unit_id.value=available.some(u=>u.unit_id===selected)?String(selected):'';
   const course=options.courses.find(c=>c.course_id===student?.course_id);
   unitHelp.textContent=!student?'':available.length?'Units for '+course.course_name+'.':'No units are assigned to '+course.course_name+'. Add a unit under Units before enrolling this student.';
  };
  inputs.student_id.addEventListener('change',updateUnits);
  updateUnits();
  if(!existing||existing.status!=='Completed'){for(const o of [...inputs.status.options])if(o.value==='Completed')o.remove();}
 }
 if(resource==='users')inputs.student_id.addEventListener('change',()=>{const s=options.students.find(s=>s.student_id===Number(inputs.student_id.value));if(s){inputs.full_name.value=s.first_name+' '+s.last_name;inputs.email.value=s.email;inputs.username.value='s'+s.student_number;}});
 const save=h('button',{class:'btn primary',type:'submit'},existing?'Save changes':'Create '+def.singular);form.append(grid,error,h('div',{class:'form-actions'},btn('Cancel',onCancel),save));
 form.addEventListener('submit',async event=>{event.preventDefault();setMessage(error,'');const payload={};for(const [name,,type] of def.fields){const value=inputs[name].value;if(type==='password'&&existing&&value==='')continue;payload[name]=['number','decimal','courses','students','accountStudents','units','enrolments'].includes(type)||name==='semester'?Number(value):name==='is_active'?value==='Active':type==='password'?value:value.trim();}if(resource==='users')payload.role='Student';save.disabled=true;save.textContent='Saving…';try{await api(resource+(existing?'/'+existing[def.pk]:''),{method:existing?'PUT':'POST',body:payload});state.lookups=null;notify((def.singular[0].toUpperCase()+def.singular.slice(1))+' saved.');await onSaved();}catch(e){setMessage(error,e.message);error.scrollIntoView({block:'nearest'});}finally{save.disabled=false;save.textContent=existing?'Save changes':'Create '+def.singular;}});return form;
}

function confirmDelete(resource,row,onDeleted){const def=RESOURCES[resource];const label=row.student_number||row.username||row.course_code||row.unit_code||('#'+row[def.pk]);$('confirm-title').textContent='Delete '+def.singular+'?';$('confirm-message').textContent='Delete '+label+'? This action cannot be undone.'+(resource==='enrolments'?' Any result attached to this enrolment will also be deleted.':'')+(resource==='results'?' The enrolment will return to Enrolled and the weighted average mark will be recalculated.':'');setMessage($('delete-error'),'');$('delete-confirm').disabled=false;$('delete-confirm').textContent='Delete '+def.singular;$('delete-confirm').onclick=async()=>{$('delete-confirm').disabled=true;try{await api(resource+'/'+row[def.pk],{method:'DELETE'});state.lookups=null;$('confirm-dialog').close();notify('Record deleted.');await onDeleted();}catch(e){setMessage($('delete-error'),e.message);}finally{$('delete-confirm').disabled=false;}};$('confirm-dialog').showModal();$('delete-cancel').focus();}

function passwordPage(){const form=h('form',{class:'record-form'}),grid=h('div',{class:'form-grid'}),inputs={};for(const [key,label] of [['current_password','Current password'],['new_password','New password'],['confirm_password','Confirm new password']]){const input=h('input',{id:key,name:key,type:'password',required:true,maxLength:200,autocomplete:key==='current_password'?'current-password':'new-password'});if(key!=='current_password')input.minLength=10;inputs[key]=input;grid.append(h('div',{class:'field'},h('label',{htmlFor:key},label),input,key==='new_password'&&h('small',{},'Use at least 10 characters.')));}const error=h('p',{class:'form-message',role:'alert',hidden:true}),save=h('button',{class:'btn primary',type:'submit'},'Change password');form.append(h('p',{class:'field-note'},'After changing your password, you will need to sign in again.'),grid,error,h('div',{class:'form-actions'},save));form.addEventListener('submit',async e=>{e.preventDefault();setMessage(error,'');if(inputs.new_password.value!==inputs.confirm_password.value){setMessage(error,'The new passwords do not match.');return;}save.disabled=true;try{await api('password',{method:'POST',body:{current_password:inputs.current_password.value,new_password:inputs.new_password.value}});showLogin('Password changed. Sign in with your new password.',true);}catch(err){setMessage(error,err.message);}finally{save.disabled=false;}});return h('div',{},heading('Change password','Keep your account access up to date.'),h('section',{class:'panel password-panel'},form));}

$('login-form').addEventListener('submit',async e=>{e.preventDefault();const submit=e.target.querySelector('button');submit.disabled=true;setMessage($('login-message'),'');try{await api('login',{method:'POST',body:{username:$('username').value.trim(),password:$('password').value}});location.hash='dashboard';openApp(await api('me'));}catch(error){setMessage($('login-message'),error.message);}finally{submit.disabled=false;}});
$('logout').prepend(icon('logout'));
$('logout').addEventListener('click',async()=>{try{await api('logout',{method:'POST'});showLogin();}catch(error){notify(error.message,true);}});
$('menu-toggle').append(icon('menu'));$('editor-close').append(icon('close'));
$('menu-toggle').addEventListener('click',()=>{const open=$('sidebar').classList.toggle('open');$('nav-backdrop').hidden=!open;$('menu-toggle').setAttribute('aria-expanded',String(open));});
$('nav-backdrop').addEventListener('click',closeNavigation);
$('editor-close').addEventListener('click',()=>$('editor').close());$('delete-cancel').addEventListener('click',()=>$('confirm-dialog').close());
document.addEventListener('keydown',e=>{if(e.key==='Escape')closeNavigation();});
window.addEventListener('hashchange',()=>{if(state.user)renderRoute();});
(async()=>{try{openApp(await api('me'));}catch{showLogin();}})();

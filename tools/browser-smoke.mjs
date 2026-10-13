// Dependency-free browser smoke test using Chrome's local debugging protocol.
// Run against a dedicated sample database; see docs/TESTING.md.
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import assert from 'node:assert/strict';

const debug = process.env.BROWSER_DEBUG_URL || 'http://127.0.0.1:9227';
const base = process.env.APP_URL || 'http://127.0.0.1:8081';
const output = process.env.SCREENSHOT_DIR || join(process.cwd(), 'docs', 'screenshots');
await mkdir(output, { recursive: true });
const target = await (await fetch(debug + '/json/new?' + encodeURIComponent(base), { method: 'PUT' })).json();
const ws = new WebSocket(target.webSocketDebuggerUrl);
await new Promise((resolve, reject) => { ws.addEventListener('open', resolve, { once: true }); ws.addEventListener('error', reject, { once: true }); });
let sequence = 0;
const pending = new Map();
const exceptions = [];
ws.addEventListener('message', event => {
  const message = JSON.parse(event.data);
  if (message.method === 'Runtime.exceptionThrown') exceptions.push(message.params.exceptionDetails.text);
  if (message.id && pending.has(message.id)) {
    const { resolve, reject, timer } = pending.get(message.id); pending.delete(message.id); clearTimeout(timer);
    message.error ? reject(Error(JSON.stringify(message.error))) : resolve(message.result);
  }
});
function command(method, params = {}) {
  return new Promise((resolve, reject) => {
    const id = ++sequence;
    const timer = setTimeout(() => { pending.delete(id); reject(Error('Timed out: ' + method)); }, 20000);
    pending.set(id, { resolve, reject, timer }); ws.send(JSON.stringify({ id, method, params }));
  });
}
async function evaluate(expression) {
  const result = await command('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
  if (result.exceptionDetails) throw Error(JSON.stringify(result.exceptionDetails));
  return result.result.value;
}
async function wait(expression, label = expression) {
  for (let i = 0; i < 100; i++) {
    if (await evaluate(expression)) return;
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw Error('Not ready: ' + label + '\n' + await evaluate('document.body.innerText'));
}
async function fill(selector, value) {
  await evaluate(`(() => { const e = document.querySelector(${JSON.stringify(selector)}); if(!e) throw Error('Missing input'); e.value = ${JSON.stringify(String(value))}; e.dispatchEvent(new Event('input',{bubbles:true})); e.dispatchEvent(new Event('change',{bubbles:true})); })()`);
}
async function click(selector) { await evaluate(`document.querySelector(${JSON.stringify(selector)}).click()`); }
async function page(route, title) {
  await evaluate(`location.hash=${JSON.stringify(route)}`);
  await wait(`document.querySelector('#content h1')?.textContent === ${JSON.stringify(title)}`);
}
async function login(username, password, title) {
  await wait('!document.querySelector("#login-screen").hidden');
  await fill('#username', username); await fill('#password', password); await evaluate('document.querySelector("#login-form").requestSubmit()');
  await wait(`document.querySelector('#content h1')?.textContent === ${JSON.stringify(title)}`);
}
async function logout() { await click('#logout'); await wait('!document.querySelector("#login-screen").hidden'); }
async function screenshot(name) { const shot = await command('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await writeFile(join(output, name + '.png'), Buffer.from(shot.data, 'base64')); }
async function read(path) { return evaluate(`fetch('/api/'+${JSON.stringify(path)}).then(r=>r.json())`); }
async function form(resource, values) {
  await evaluate(`Array.from(document.querySelectorAll('#content button')).find(b => b.textContent.trim()===${JSON.stringify('Add ' + resource)}).click()`);
  await wait('document.querySelector("#editor").open');
  for (const [key, value] of Object.entries(values)) await fill('#record-' + key, value);
  await evaluate('document.querySelector("#editor form").requestSubmit()');
  await wait('!document.querySelector("#editor").open', 'record saved');
}
async function deleteMatch(route, title, search) {
  await page(route, title);
  await fill('#content input[type=search]', search);
  await wait(`document.querySelectorAll('#content tbody tr').length===1 && document.querySelector('#content tbody tr').innerText.includes(${JSON.stringify(search)})`);
  await click('#content .danger-text'); await wait('document.querySelector("#confirm-dialog").open'); await click('#delete-confirm');
  await wait('!document.querySelector("#confirm-dialog").open', 'record deleted');
}

try {
  await command('Page.enable'); await command('Runtime.enable');
  await command('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
  await command('Page.navigate', { url: base });
  await wait('document.querySelector("#login-screen") && !document.querySelector("#login-screen").hidden');
  await screenshot('01-login');
  await login('admin', 'Admin@12345', 'Dashboard');
  await screenshot('02-admin-dashboard');
  await page('enrolments', 'Enrolments');
  await evaluate("Array.from(document.querySelectorAll('#content button')).find(b=>b.textContent.trim()==='Add enrolment').click()");
  await wait('document.querySelector("#editor").open');
  assert.equal(await evaluate('document.querySelector("#record-unit_id").disabled'), true);
  await fill('#record-student_id', 1);
  assert.equal(await evaluate('document.querySelector("#record-unit_id").options.length'), 5);
  await fill('#record-unit_id', 1);
  await fill('#record-student_id', 3);
  assert.equal(await evaluate('document.querySelector("#record-unit_id").options.length'), 4);
  assert.equal(await evaluate('document.querySelector("#record-unit_id").value'), '');
  await fill('#record-unit_id', 5);
  assert.equal(await evaluate('document.querySelector("#record-unit_id").value'), '5');
  await screenshot('07-enrolment-form');
  await click('#editor-close');
  await page('students', 'Students'); await screenshot('03-student-management');
  await page('courses', 'Courses');
  await form('course', { course_code: 'QA-COURSE', course_name: 'Quality Assurance Studies', duration_years: 3, description: 'Faculty of Computing' });
  const course = (await read('courses?q=QA-COURSE')).items[0]; assert.ok(course);
  await page('units', 'Units');
  await form('unit', { unit_code: 'QA-UNIT', unit_name: 'Applied Research', course_id: course.course_id, credit_points: 20 });
  const unit = (await read('units?q=QA-UNIT')).items[0]; assert.ok(unit);
  await page('students/new', 'Add student');
  for (const [key, value] of Object.entries({ student_number: 'QA9001', course_id: course.course_id, first_name: 'Maya', last_name: 'Test', email: 'maya.test@example.edu', phone: '0771234567', date_of_birth: '2003-05-12', address: 'Colombo' })) await fill('#record-' + key, value);
  await evaluate('document.querySelector("#content form").requestSubmit()');
  await wait('document.querySelector("#content h1")?.textContent==="Students"');
  const student = (await read('students?q=QA9001')).items[0]; assert.ok(student);
  await page('students/' + student.student_id + '/edit', 'Edit student');
  await fill('#record-phone', '0777654321'); await evaluate('document.querySelector("#content form").requestSubmit()');
  await wait('document.querySelector("#content h1")?.textContent==="Student academic summary"');
  assert.equal((await read('students/' + student.student_id)).phone, '0777654321');
  await page('enrolments', 'Enrolments');
  await form('enrolment', { student_id: student.student_id, unit_id: unit.unit_id, academic_year: 2026, semester: 2, enrolment_date: '2026-08-03' });
  const enrolment = (await read('enrolments?q=QA9001')).items[0]; assert.ok(enrolment);
  await page('results', 'Results'); await form('result', { enrolment_id: enrolment.enrolment_id, mark: '84.5' });
  assert.equal((await read('students/' + student.student_id + '/summary')).wam, 84.5);
  await page('users', 'Student accounts'); await form('account', { student_id: student.student_id, password: 'StudentTest!123' });
  await logout(); await login('sQA9001', 'StudentTest!123', 'My dashboard');
  assert.equal(await evaluate('document.querySelectorAll("#navigation a").length'), 6);
  await page('profile', 'My profile'); assert.ok((await evaluate('document.querySelector("#content").innerText')).includes('Maya Test'));
  await page('enrolments', 'My enrolments'); await page('results', 'My results');
  assert.ok((await evaluate('document.querySelector("#content").innerText')).includes('84.50'));
  await page('summary', 'Academic summary');
  await page('password', 'Change password');
  await fill('#current_password', 'StudentTest!123'); await fill('#new_password', 'StudentChanged!123'); await fill('#confirm_password', 'StudentChanged!123');
  await evaluate('document.querySelector("#content form").requestSubmit()');await wait('!document.querySelector("#login-screen").hidden');
  await login('sQA9001', 'StudentChanged!123', 'My dashboard'); await logout();
  await login('s2026001', 'Student@12345', 'My dashboard'); await screenshot('04-student-dashboard');
  await page('summary', 'Academic summary'); await screenshot('05-academic-summary');
  await command('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 1, mobile: true });
  await page('dashboard', 'My dashboard');
  assert.equal(await evaluate('document.documentElement.scrollWidth <= innerWidth'), true, 'mobile page overflows');
  await click('#menu-toggle'); assert.equal(await evaluate('document.querySelector("#sidebar").classList.contains("open")'), true); await click('#nav-backdrop');
  await screenshot('06-mobile-student-dashboard');
  await command('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
  await logout(); await login('admin', 'Admin@12345', 'Dashboard');
  await deleteMatch('users', 'Student accounts', 'sQA9001');
  await deleteMatch('results', 'Results', 'QA9001');
  await deleteMatch('enrolments', 'Enrolments', 'QA9001');
  await deleteMatch('students', 'Students', 'QA9001');
  await deleteMatch('units', 'Units', 'QA-UNIT');
  await deleteMatch('courses', 'Courses', 'QA-COURSE');
  await page('dashboard', 'Dashboard');
  assert.equal((await read('dashboard')).counts.students, 8);
  assert.deepEqual(exceptions, [], 'browser runtime errors');
  console.log('PASS: Admin CRUD, student account creation, student portal, password change, search, deletion, mobile navigation and no browser runtime errors.');
  console.log('Screenshots: ' + output);
} finally { ws.close(); }

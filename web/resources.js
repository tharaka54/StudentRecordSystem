'use strict';
// Declarative forms keep academic resources consistent. The server validates every field again.
const RESOURCES = {
 students: {title:'Students',singular:'student',description:'Manage student profiles and course registrations.',pk:'student_id',search:'Search name, student number or email',
  columns:[['student_number','Student number'],['student_name','Student'],['course_code','Course'],['status','Status']],
  fields:[['student_number','Student number','text',true,30],['course_id','Course','courses',true],['first_name','First name','text',true,100],['last_name','Last name','text',true,100],['email','Email address','email',true,200],['phone','Phone number','tel',false,30],['date_of_birth','Date of birth','date',false],['status','Status','options',true,['Active','Inactive']],['address','Address','textarea',false,300]]},
 courses: {title:'Courses',singular:'course',description:'Maintain course information and study duration.',pk:'course_id',search:'Search course code or name',
  columns:[['course_code','Course code'],['course_name','Course name'],['duration_years','Duration'],['description','School / description']],
  fields:[['course_code','Course code','text',true,30],['course_name','Course name','text',true,120],['duration_years','Duration (years)','number',true,[1,10]],['description','Description','textarea',false,500]]},
 units: {title:'Units',singular:'unit',description:'Manage the unit catalogue and credit points.',pk:'unit_id',search:'Search unit code, title or course',
  columns:[['unit_code','Unit code'],['unit_name','Unit name'],['course_code','Course'],['credit_points','Credit points']],
  fields:[['unit_code','Unit code','text',true,30],['unit_name','Unit name','text',true,120],['course_id','Course','courses',true],['credit_points','Credit points','number',true,[1,100]]]},
 enrolments: {title:'Enrolments',singular:'enrolment',description:'Manage unit registrations by student and academic period.',pk:'enrolment_id',search:'Search student or unit',
  columns:[['student_name','Student'],['unit_code','Unit'],['term','Academic period'],['status','Status']],
  fields:[['student_id','Student','students',true],['unit_id','Unit','units',true],['academic_year','Academic year','number',true,[2000,2100]],['semester','Semester','options',true,['1','2','3']],['enrolment_date','Enrolment date','date',true],['status','Status','options',true,['Enrolled','Completed','Withdrawn']]]},
 results: {title:'Results',singular:'result',description:'Record and review marks for enrolled units.',pk:'result_id',search:'Search student, unit or grade',
  columns:[['student_name','Student'],['unit_code','Unit'],['term','Academic period'],['mark','Mark'],['grade','Grade']],
  fields:[['enrolment_id','Student and enrolment','enrolments',true],['mark','Mark (0–100)','decimal',true,[0,100]]]},
 users: {title:'Student accounts',singular:'account',description:'Manage student access and reset account passwords.',pk:'user_id',search:'Search username, student name or email',
  columns:[['username','Username'],['full_name','Student'],['email','Email address'],['is_active','Account status']],
  fields:[['student_id','Linked student','accountStudents',true],['username','Username','text',true,50],['full_name','Account name','text',true,100],['email','Email address','email',true,200],['is_active','Account status','options',true,['Active','Disabled']],['password','Password','password',false,200]]}
};

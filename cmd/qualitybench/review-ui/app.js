'use strict';
const $=id=>document.getElementById(id), M=ReviewModel;
let manifest, reviewer='', ratings={}, index=0, pinned=null, storageOK=true, conflicted=false, loadedID=null;
const definitions=[['readability','Readability','Text is legible, unclipped, and comfortable to read.'],['hierarchy','Visual hierarchy','The key message is clear; layout guides your attention.'],['template_fidelity','Template fidelity','Brand, colors, typography, and slide chrome are consistent.'],['factual_completeness','Factual completeness','Content is complete and internally consistent; no key facts appear lost.'],['usability','Overall usability','Could you use this deck with minimal editing?']];
function notify(text,error=false){$('message').textContent=text;$('message').classList.toggle('error',error);}
function key(){return 'qualitybench-review:'+manifest.dataset+':'+reviewer;}
function packet(){return {version:1,dataset:manifest.dataset,reviewer,index,ratings};}
function save(){if(!reviewer)return;if(conflicted){notify('This profile changed in another tab. Download a backup and reopen your profile before editing.',true);return;}try{localStorage.setItem(key(),JSON.stringify(packet()));storageOK=true;notify('Saved in this browser for '+reviewer+'. Download a backup regularly.');}catch(e){storageOK=false;notify('Browser storage is unavailable. Your changes remain in memory—download a backup before closing this page.',true);}}
function field(name,title,description,choices){
  const f=document.createElement('fieldset');f.className='metric';const legend=document.createElement('legend');legend.textContent=title;f.append(legend);
  const hint=document.createElement('small');hint.textContent=description;f.append(hint);const row=document.createElement('div');row.className='choices';
  for(const [value,text] of choices){const l=document.createElement('label'),input=document.createElement('input'),s=document.createElement('span');input.type='radio';input.name=name;input.value=value;s.textContent=text;l.append(input,s);row.append(l);}
  f.append(row);return f;
}
for(const [name,title,description] of definitions)$('metrics').append(field(name,title,description,[1,2,3,4,5].map(n=>[n,n])));
$('defects').append(field('lost_critical_fact','Critical fact lost?','Missing or altered information that changes the meaning.',[[false,'No'],[true,'Yes']]));
$('defects').append(field('critical_template_defect','Critical template defect?','Broken branding, layout, or slide structure.',[[false,'No'],[true,'Yes']]));
function render(){
  const id=manifest.ids[index],r=ratings[id]||{},done=manifest.ids.filter(id=>M.complete(ratings[id])).length;
  $('currentLabel').textContent='Deck '+(index+1)+' / '+manifest.ids.length+' · '+id;
  if($('currentImage').getAttribute('src')!=='/sheet/'+id){loadedID=null;$('currentImage').src='/sheet/'+id;}
  $('brief').textContent=manifest.prompts[id]||'Inspect the content for consistency and completeness.';
  $('paired').hidden=!manifest.pairs[id];
  $('progressText').textContent=done+' of '+manifest.ids.length+' rated'+(reviewer?' · '+reviewer:'');$('progress').max=manifest.ids.length;$('progress').value=done;
  $('jump').value=id;for(const o of $('jump').options)o.textContent=(manifest.ids.indexOf(o.value)+1)+'. '+o.value+(M.complete(ratings[o.value])?' ✓':'');
  for(const input of $('ratings').elements){if(input.type==='radio'){input.disabled=!reviewer||conflicted||loadedID!==id;input.checked=String(r[input.name])===input.value;}}
  for(const id of ['backup','export','saveNext','importFile'])$(id).disabled=!reviewer;
  if(conflicted){$('export').disabled=true;$('saveNext').disabled=true;$('importFile').disabled=true;}
  $('previous').disabled=index===0;$('next').disabled=index===manifest.ids.length-1;
  $('reference').style.display=pinned===null?'none':'block';if(pinned!==null){$('referenceImage').src='/sheet/'+manifest.ids[pinned];$('referenceLabel').textContent='Pinned · '+manifest.ids[pinned];}
}
$('ratings').addEventListener('change',e=>{if(!reviewer||conflicted||loadedID!==manifest.ids[index])return;const r=ratings[manifest.ids[index]]||{};r[e.target.name]=M.scores.includes(e.target.name)?Number(e.target.value):e.target.value==='true';ratings[manifest.ids[index]]=r;save();render();});
function move(i){index=i;save();render();}
$('previous').addEventListener('click',()=>move(Math.max(0,index-1)));$('next').addEventListener('click',()=>move(Math.min(manifest.ids.length-1,index+1)));$('jump').addEventListener('change',()=>move(manifest.ids.indexOf($('jump').value)));
$('unrated').addEventListener('click',()=>move(M.nextUnrated(manifest.ids,ratings,index)));
$('saveNext').addEventListener('click',()=>{if(!M.complete(ratings[manifest.ids[index]])){notify('Choose all five scores and both defect answers before moving on.',true);return;}move(M.nextUnrated(manifest.ids,ratings,index));});
$('pin').addEventListener('click',()=>{pinned=index;render();});$('unpin').addEventListener('click',()=>{pinned=null;render();});
$('paired').addEventListener('click',()=>{pinned=manifest.ids.indexOf(manifest.pairs[manifest.ids[index]]);render();});
$('rateReference').addEventListener('click',()=>{const previous=index;index=pinned;pinned=previous;save();render();});
function download(name,content,type){const url=URL.createObjectURL(new Blob([content],{type})),a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}
function safeName(){return reviewer.replace(/[^a-zA-Z0-9_-]/g,'_');}
$('backup').addEventListener('click',()=>download('review-'+safeName()+'-backup.json',JSON.stringify(packet(),null,2),'application/json'));
$('export').addEventListener('click',()=>{try{download('ratings-'+safeName()+'.csv',M.csv(manifest.ids,reviewer,ratings),'text/csv');notify('Finished CSV downloaded. Send this file to the benchmark operator.');}catch(e){notify(e.message,true);}});
$('importFile').addEventListener('change',async()=>{const file=$('importFile').files[0];if(!file)return;try{const data=M.validateBackup(JSON.parse(await file.text()),manifest.dataset,manifest.ids,reviewer);if(Object.keys(ratings).length && !confirm('Replace this reviewer’s current progress with the backup?'))return;ratings=data.ratings;index=data.index;save();render();}catch(e){notify('Cannot restore: '+e.message,true);}finally{$('importFile').value='';}});
$('load').addEventListener('click',()=>{const name=$('reviewer').value.trim();if(!name){notify('Enter a reviewer name first.',true);return;}if(reviewer && !storageOK && !confirm('Progress is not saved. Download a backup before switching. Continue?'))return;
  let data=null,raw=null;try{raw=localStorage.getItem('qualitybench-review:'+manifest.dataset+':'+name);}catch(e){storageOK=false;}
  if(raw){try{data=M.validateBackup(JSON.parse(raw),manifest.dataset,manifest.ids,name);}catch(e){notify('Cannot load saved progress: '+e.message+'. Your current review has not changed; download or restore a backup.',true);return;}}
  reviewer=name;ratings=data?data.ratings:{};index=data?data.index:0;pinned=null;conflicted=false;save();render();});
for(const imageID of ['currentImage','referenceImage']){$(imageID).addEventListener('click',()=>{$('zoomImage').src=$(imageID).src;$('zoom').showModal();});$(imageID).addEventListener('error',()=>notify('Contact sheet could not be loaded. Do not score it; check the server and refresh.',true));}
$('currentImage').addEventListener('load',()=>{loadedID=manifest.ids[index];render();});
$('closeZoom').addEventListener('click',()=>$('zoom').close());
window.addEventListener('beforeunload',e=>{if(reviewer&&!storageOK){e.preventDefault();e.returnValue='';}});
window.addEventListener('storage',e=>{if(reviewer && e.key===key()){conflicted=true;render();notify('This reviewer was updated in another tab. Download a backup, then reopen the profile to load that change.',true);}});
for(const id of ['load','previous','next','jump','pin','paired','unrated','backup','export','saveNext','importFile'])$(id).disabled=true;
fetch('/manifest').then(r=>{if(!r.ok)throw Error('Manifest could not load');return r.json();}).then(data=>{manifest=data;for(const id of manifest.ids){const o=document.createElement('option');o.value=id;$('jump').append(o);}if(manifest.failed_runs){$('datasetWarning').hidden=false;$('datasetWarning').textContent=manifest.failed_runs+' runs failed regeneration and are excluded here. You can rate the available sheets, but this incomplete bundle cannot approve a release.';}render();for(const id of ['load','jump','pin','paired','unrated'])$(id).disabled=false;}).catch(e=>{notify(e.message+'. Keep the review server running and reload.',true);});

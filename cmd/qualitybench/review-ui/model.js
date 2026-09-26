/* Pure ballot logic: shared by the browser and dependency-free Node tests. */
(function(root) {
  'use strict';
  const scores = ['readability','hierarchy','template_fidelity','factual_completeness','usability'];
  const defects = ['lost_critical_fact','critical_template_defect'];
  const complete = r => !!r && scores.every(k => Number.isInteger(r[k]) && r[k]>=1 && r[k]<=5) && defects.every(k => typeof r[k]==='boolean');
  const quote = v => '"'+String(v).replaceAll('"','""')+'"';
  function csv(ids, reviewer, ratings) {
    if (!reviewer.trim() || ids.some(id=>!complete(ratings[id]))) throw Error('Complete every sheet before exporting the final CSV.');
    const header = ['blind_id','reviewer','reviewer_type',...scores,...defects];
    return header.join(',')+'\r\n'+ids.map(id=>[id,reviewer,'human',...scores.map(k=>ratings[id][k]),...defects.map(k=>ratings[id][k])].map(quote).join(',')).join('\r\n')+'\r\n';
  }
  function validateBackup(data, dataset, ids, reviewer) {
    if (!data || data.version!==1 || data.dataset!==dataset || data.reviewer!==reviewer || !data.ratings || typeof data.ratings!=='object' || Array.isArray(data.ratings)) throw Error('This backup belongs to another dataset or reviewer. Open the matching reviewer first.');
    const known = new Set(ids);
    for (const [id,r] of Object.entries(data.ratings)) {
      if (!known.has(id) || !r || typeof r!=='object' || Array.isArray(r)) throw Error('Backup contains an unknown or invalid sheet.');
      for (const [k,v] of Object.entries(r)) {
        if (scores.includes(k) ? !(Number.isInteger(v)&&v>=1&&v<=5) : defects.includes(k) ? typeof v!=='boolean' : true) throw Error('Backup contains invalid scores or fields.');
      }
    }
    if (!Number.isInteger(data.index) || data.index<0 || data.index>=ids.length) throw Error('Backup has an invalid position.');
    return data;
  }
  function nextUnrated(ids, ratings, index) {
    for(let n=1;n<=ids.length;n++){ const i=(index+n)%ids.length;if(!complete(ratings[ids[i]]))return i; }
    return index;
  }
  const api = {scores,defects,complete,csv,validateBackup,nextUnrated};
  if(typeof module!=='undefined' && module.exports)module.exports=api;else root.ReviewModel=api;
})(typeof globalThis!=='undefined'?globalThis:this);

import React, { useState, useEffect, useCallback, useRef } from 'react';
import { learningAPI } from '../../services/api';
import { useFetch } from '../../hooks/useFetch';
import type { LearningSubject, LearningChapter, KnowledgePoint, PaginatedResponse } from '../../types';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';
import EmptyState from '../common/EmptyState';
import Pagination from '../common/Pagination';

const inputStyle: React.CSSProperties = {
  width: '100%', padding: '0.5rem', border: '1px solid #ccc', borderRadius: 4, fontSize: '0.9rem', boxSizing: 'border-box',
};
const btnPrimary: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#1976d2', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const btnSecondary: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#6c757d', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const btnDisabled: React.CSSProperties = { ...btnPrimary, opacity: 0.6, cursor: 'not-allowed' };
const panelStyle: React.CSSProperties = {
  flex: 1, borderRight: '1px solid #ddd', padding: '1rem', overflowY: 'auto', minWidth: 0,
};
const listItemStyle = (selected: boolean): React.CSSProperties => ({
  padding: '0.5rem 0.75rem', cursor: 'pointer', borderRadius: 4, marginBottom: 2,
  backgroundColor: selected ? '#e3f2fd' : 'transparent', borderLeft: selected ? '3px solid #1976d2' : '3px solid transparent',
});
const successStyle: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#d4edda', border: '1px solid #c3e6cb', borderRadius: 4, color: '#155724', marginBottom: '0.5rem',
};

const LearningPage: React.FC = () => {
  // Subjects
  const { data: subjects, loading: subjectsLoading, error: subjectsError, refetch: refetchSubjects } = useFetch<LearningSubject[]>(
    () => learningAPI.listSubjects().then(r => ({ data: r.data.data || r.data })), []
  );
  const [selectedSubject, setSelectedSubject] = useState<LearningSubject | null>(null);
  const [showSubjectForm, setShowSubjectForm] = useState(false);
  const [subjectForm, setSubjectForm] = useState({ name: '', description: '', sort_order: 0 });
  const [subjectFormErr, setSubjectFormErr] = useState('');
  const [subjectSubmitting, setSubjectSubmitting] = useState(false);
  const [subjectSuccess, setSubjectSuccess] = useState('');

  // Chapters
  const [chapters, setChapters] = useState<LearningChapter[]>([]);
  const [chaptersLoading, setChaptersLoading] = useState(false);
  const [chaptersError, setChaptersError] = useState('');
  const [selectedChapter, setSelectedChapter] = useState<LearningChapter | null>(null);
  const [showChapterForm, setShowChapterForm] = useState(false);
  const [chapterForm, setChapterForm] = useState({ name: '', sort_order: 0 });
  const [chapterFormErr, setChapterFormErr] = useState('');
  const [chapterSubmitting, setChapterSubmitting] = useState(false);
  const [chapterSuccess, setChapterSuccess] = useState('');

  // Knowledge Points
  const [kps, setKps] = useState<KnowledgePoint[]>([]);
  const [kpTotal, setKpTotal] = useState(0);
  const [kpPage, setKpPage] = useState(1);
  const [kpsLoading, setKpsLoading] = useState(false);
  const [kpsError, setKpsError] = useState('');
  const [showKpForm, setShowKpForm] = useState(false);
  const [editingKp, setEditingKp] = useState<KnowledgePoint | null>(null);
  const [kpForm, setKpForm] = useState({ title: '', content: '', tags: '', classifications: '{}' });
  const [kpFormErr, setKpFormErr] = useState('');
  const [kpSubmitting, setKpSubmitting] = useState(false);
  const [kpSuccess, setKpSuccess] = useState('');

  // Search
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<KnowledgePoint[] | null>(null);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchError, setSearchError] = useState('');
  const [searchTotal, setSearchTotal] = useState(0);
  const [searchPage, setSearchPage] = useState(1);

  // Import modal state
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [showImportModal, setShowImportModal] = useState(false);
  const [importForm, setImportForm] = useState({ category: '', title: '', chapter_id: '' });
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importFormErr, setImportFormErr] = useState('');
  const [importLoading, setImportLoading] = useState(false);
  const [importMsg, setImportMsg] = useState('');

  // Fetch chapters when subject changes
  useEffect(() => {
    if (!selectedSubject) { setChapters([]); setSelectedChapter(null); return; }
    setChaptersLoading(true);
    setChaptersError('');
    setSelectedChapter(null);
    learningAPI.listChapters({ subject_id: selectedSubject.id })
      .then(r => setChapters(r.data.data || r.data))
      .catch(e => setChaptersError(e.response?.data?.error || 'Failed to load chapters'))
      .finally(() => setChaptersLoading(false));
  }, [selectedSubject]);

  // Fetch KPs when chapter changes
  const fetchKps = useCallback((page = 1) => {
    if (!selectedChapter) { setKps([]); return; }
    setKpsLoading(true);
    setKpsError('');
    learningAPI.listKnowledgePoints({ chapter_id: selectedChapter.id, page, page_size: 20 })
      .then(r => {
        const d = r.data;
        setKps(d.data || d);
        setKpTotal(d.total || (d.data || d).length);
        setKpPage(page);
      })
      .catch(e => setKpsError(e.response?.data?.error || 'Failed to load knowledge points'))
      .finally(() => setKpsLoading(false));
  }, [selectedChapter]);

  useEffect(() => { fetchKps(1); }, [fetchKps]);

  // Search
  const handleSearch = useCallback(async (page = 1) => {
    if (!searchQuery.trim()) { setSearchResults(null); return; }
    setSearchLoading(true);
    setSearchError('');
    try {
      const r = await learningAPI.search({ q: searchQuery.trim(), page, page_size: 20 });
      const d = r.data;
      setSearchResults(d.data || d);
      setSearchTotal(d.total || (d.data || d).length);
      setSearchPage(page);
    } catch (e: any) {
      setSearchError(e.response?.data?.error || 'Search failed');
    } finally {
      setSearchLoading(false);
    }
  }, [searchQuery]);

  // Subject form
  const handleSubjectSubmit = async () => {
    if (!subjectForm.name.trim()) { setSubjectFormErr('Name is required'); return; }
    setSubjectSubmitting(true);
    setSubjectFormErr('');
    try {
      await learningAPI.createSubject({ name: subjectForm.name.trim(), description: subjectForm.description.trim(), sort_order: subjectForm.sort_order });
      setSubjectSuccess('Subject created successfully');
      setShowSubjectForm(false);
      setSubjectForm({ name: '', description: '', sort_order: 0 });
      refetchSubjects();
      setTimeout(() => setSubjectSuccess(''), 3000);
    } catch (e: any) {
      setSubjectFormErr(e.response?.data?.error || 'Failed to create subject');
    } finally {
      setSubjectSubmitting(false);
    }
  };

  // Chapter form
  const handleChapterSubmit = async () => {
    if (!chapterForm.name.trim()) { setChapterFormErr('Name is required'); return; }
    if (!selectedSubject) { setChapterFormErr('Select a subject first'); return; }
    setChapterSubmitting(true);
    setChapterFormErr('');
    try {
      await learningAPI.createChapter({ subject_id: selectedSubject.id, name: chapterForm.name.trim(), sort_order: chapterForm.sort_order });
      setChapterSuccess('Chapter created successfully');
      setShowChapterForm(false);
      setChapterForm({ name: '', sort_order: 0 });
      // refetch chapters
      const r = await learningAPI.listChapters({ subject_id: selectedSubject.id });
      setChapters(r.data.data || r.data);
      setTimeout(() => setChapterSuccess(''), 3000);
    } catch (e: any) {
      setChapterFormErr(e.response?.data?.error || 'Failed to create chapter');
    } finally {
      setChapterSubmitting(false);
    }
  };

  // KP form
  const openKpForm = (kp?: KnowledgePoint) => {
    if (kp) {
      setEditingKp(kp);
      setKpForm({
        title: kp.title, content: kp.content,
        tags: (kp.tags || []).join(', '),
        classifications: JSON.stringify(kp.classifications || {}, null, 2),
      });
    } else {
      setEditingKp(null);
      setKpForm({ title: '', content: '', tags: '', classifications: '{}' });
    }
    setKpFormErr('');
    setShowKpForm(true);
  };

  const handleKpSubmit = async () => {
    if (!kpForm.title.trim()) { setKpFormErr('Title is required'); return; }
    if (!kpForm.content.trim()) { setKpFormErr('Content is required'); return; }
    let classifications: Record<string, unknown>;
    try { classifications = JSON.parse(kpForm.classifications); }
    catch { setKpFormErr('Classifications must be valid JSON'); return; }
    if (!selectedChapter && !editingKp) { setKpFormErr('Select a chapter first'); return; }

    setKpSubmitting(true);
    setKpFormErr('');
    const tags = kpForm.tags.split(',').map(t => t.trim()).filter(Boolean);
    try {
      if (editingKp) {
        await learningAPI.updateKnowledgePoint(editingKp.id, { title: kpForm.title.trim(), content: kpForm.content, tags, classifications });
      } else {
        await learningAPI.createKnowledgePoint({ chapter_id: selectedChapter!.id, title: kpForm.title.trim(), content: kpForm.content, tags, classifications });
      }
      setKpSuccess(editingKp ? 'Knowledge point updated' : 'Knowledge point created');
      setShowKpForm(false);
      fetchKps(kpPage);
      setTimeout(() => setKpSuccess(''), 3000);
    } catch (e: any) {
      setKpFormErr(e.response?.data?.error || 'Failed to save knowledge point');
    } finally {
      setKpSubmitting(false);
    }
  };

  // Import modal handlers
  const openImportModal = () => {
    setImportForm({ category: '', title: '', chapter_id: selectedChapter?.id || '' });
    setImportFile(null);
    setImportFormErr('');
    setShowImportModal(true);
  };

  const handleImportFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] || null;
    setImportFile(file);
  };

  const handleImportSubmit = async () => {
    if (!importForm.category.trim()) { setImportFormErr('Category is required'); return; }
    if (!importForm.title.trim()) { setImportFormErr('Title is required'); return; }
    if (!importForm.chapter_id.trim()) { setImportFormErr('Chapter ID is required — select a chapter first or paste its ID'); return; }
    if (!importFile) { setImportFormErr('A file is required'); return; }
    setImportLoading(true);
    setImportFormErr('');
    const fd = new FormData();
    fd.append('category', importForm.category.trim());
    fd.append('title', importForm.title.trim());
    fd.append('chapter_id', importForm.chapter_id.trim());
    fd.append('file', importFile);
    try {
      await learningAPI.importContent(fd);
      setImportMsg('Import successful');
      setShowImportModal(false);
      refetchSubjects();
      setTimeout(() => setImportMsg(''), 3000);
    } catch (err: any) {
      setImportFormErr('Import failed: ' + (err.response?.data?.error || err.message));
    } finally {
      setImportLoading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  // Export
  const handleExport = async (kpId: string) => {
    try {
      const r = await learningAPI.exportContent(kpId);
      const url = window.URL.createObjectURL(new Blob([r.data]));
      const a = document.createElement('a');
      a.href = url;
      a.download = `knowledge-point-${kpId}.md`;
      a.click();
      window.URL.revokeObjectURL(url);
    } catch (err: any) {
      alert('Export failed: ' + (err.response?.data?.error || err.message));
    }
  };

  const displayKps = searchResults !== null ? searchResults : kps;
  const displayTotal = searchResults !== null ? searchTotal : kpTotal;
  const displayPage = searchResults !== null ? searchPage : kpPage;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Top bar */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', padding: '1rem', borderBottom: '1px solid #ddd', flexWrap: 'wrap' }}>
        <h2 style={{ margin: 0, marginRight: 'auto' }}>Knowledge Base</h2>
        <input
          type="text" placeholder="Search knowledge points..." value={searchQuery}
          onChange={e => { setSearchQuery(e.target.value); if (!e.target.value.trim()) setSearchResults(null); }}
          onKeyDown={e => e.key === 'Enter' && handleSearch(1)}
          style={{ ...inputStyle, width: 280 }}
        />
        <button onClick={() => handleSearch(1)} style={btnPrimary} disabled={searchLoading}>
          {searchLoading ? 'Searching...' : 'Search'}
        </button>
        <button onClick={() => { setSearchQuery(''); setSearchResults(null); }} style={btnSecondary}>Clear</button>
        <input ref={fileInputRef} type="file" accept=".md,.html,.htm" style={{ display: 'none' }} onChange={handleImportFileChange} />
        <button onClick={openImportModal} style={btnSecondary} disabled={importLoading}>
          {importLoading ? 'Importing...' : 'Import'}
        </button>
      </div>
      {importMsg && <div style={{ padding: '0.5rem 1rem', backgroundColor: importMsg.startsWith('Import failed') ? '#fdecea' : '#d4edda', color: importMsg.startsWith('Import failed') ? '#721c24' : '#155724' }}>{importMsg}</div>}
      {searchError && <ErrorMessage message={searchError} />}
      {subjectSuccess && <div style={successStyle}>{subjectSuccess}</div>}
      {chapterSuccess && <div style={successStyle}>{chapterSuccess}</div>}
      {kpSuccess && <div style={successStyle}>{kpSuccess}</div>}

      {/* Three-panel layout */}
      <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
        {/* Left: Subjects */}
        <div style={{ ...panelStyle, maxWidth: 260 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
            <strong>Subjects</strong>
            <button onClick={() => setShowSubjectForm(true)} style={{ ...btnPrimary, padding: '0.25rem 0.5rem', fontSize: '0.8rem' }}>+ Add</button>
          </div>
          {subjectsLoading && <LoadingSpinner message="Loading subjects..." />}
          {subjectsError && <ErrorMessage message={subjectsError} onRetry={refetchSubjects} />}
          {!subjectsLoading && !subjectsError && (!subjects || subjects.length === 0) && (
            <EmptyState message="No subjects yet" actionLabel="Create Subject" onAction={() => setShowSubjectForm(true)} />
          )}
          {subjects && subjects.map(s => (
            <div key={s.id} style={listItemStyle(selectedSubject?.id === s.id)} onClick={() => setSelectedSubject(s)}>
              <div style={{ fontWeight: 500 }}>{s.name}</div>
              {s.description && <div style={{ fontSize: '0.8rem', color: '#888', marginTop: 2 }}>{s.description}</div>}
            </div>
          ))}

          {showSubjectForm && (
            <div style={{ marginTop: '1rem', padding: '0.75rem', backgroundColor: '#f9f9f9', borderRadius: 4 }}>
              <h4 style={{ margin: '0 0 0.5rem' }}>New Subject</h4>
              {subjectFormErr && <div style={{ color: '#dc3545', marginBottom: '0.5rem', fontSize: '0.85rem' }}>{subjectFormErr}</div>}
              <input placeholder="Name *" value={subjectForm.name} onChange={e => setSubjectForm({ ...subjectForm, name: e.target.value })} style={{ ...inputStyle, marginBottom: '0.5rem' }} />
              <input placeholder="Description" value={subjectForm.description} onChange={e => setSubjectForm({ ...subjectForm, description: e.target.value })} style={{ ...inputStyle, marginBottom: '0.5rem' }} />
              <input type="number" placeholder="Sort order" value={subjectForm.sort_order} onChange={e => setSubjectForm({ ...subjectForm, sort_order: parseInt(e.target.value) || 0 })} style={{ ...inputStyle, marginBottom: '0.5rem' }} />
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <button onClick={handleSubjectSubmit} disabled={subjectSubmitting} style={subjectSubmitting ? btnDisabled : btnPrimary}>
                  {subjectSubmitting ? 'Creating...' : 'Create'}
                </button>
                <button onClick={() => { setShowSubjectForm(false); setSubjectFormErr(''); }} style={btnSecondary}>Cancel</button>
              </div>
            </div>
          )}
        </div>

        {/* Middle: Chapters */}
        <div style={{ ...panelStyle, maxWidth: 260 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
            <strong>Chapters</strong>
            <button onClick={() => setShowChapterForm(true)} disabled={!selectedSubject}
              style={{ ...(selectedSubject ? btnPrimary : btnDisabled), padding: '0.25rem 0.5rem', fontSize: '0.8rem' }}>+ Add</button>
          </div>
          {!selectedSubject && <div style={{ color: '#999', textAlign: 'center', padding: '2rem 0' }}>Select a subject</div>}
          {selectedSubject && chaptersLoading && <LoadingSpinner message="Loading chapters..." />}
          {selectedSubject && chaptersError && <ErrorMessage message={chaptersError} />}
          {selectedSubject && !chaptersLoading && !chaptersError && chapters.length === 0 && (
            <EmptyState message="No chapters" actionLabel="Create Chapter" onAction={() => setShowChapterForm(true)} />
          )}
          {chapters.map(c => (
            <div key={c.id} style={listItemStyle(selectedChapter?.id === c.id)} onClick={() => setSelectedChapter(c)}>
              <div style={{ fontWeight: 500 }}>{c.name}</div>
              <div style={{ fontSize: '0.8rem', color: '#888' }}>Order: {c.sort_order}</div>
            </div>
          ))}

          {showChapterForm && (
            <div style={{ marginTop: '1rem', padding: '0.75rem', backgroundColor: '#f9f9f9', borderRadius: 4 }}>
              <h4 style={{ margin: '0 0 0.5rem' }}>New Chapter</h4>
              {chapterFormErr && <div style={{ color: '#dc3545', marginBottom: '0.5rem', fontSize: '0.85rem' }}>{chapterFormErr}</div>}
              <input placeholder="Name *" value={chapterForm.name} onChange={e => setChapterForm({ ...chapterForm, name: e.target.value })} style={{ ...inputStyle, marginBottom: '0.5rem' }} />
              <input type="number" placeholder="Sort order" value={chapterForm.sort_order} onChange={e => setChapterForm({ ...chapterForm, sort_order: parseInt(e.target.value) || 0 })} style={{ ...inputStyle, marginBottom: '0.5rem' }} />
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <button onClick={handleChapterSubmit} disabled={chapterSubmitting} style={chapterSubmitting ? btnDisabled : btnPrimary}>
                  {chapterSubmitting ? 'Creating...' : 'Create'}
                </button>
                <button onClick={() => { setShowChapterForm(false); setChapterFormErr(''); }} style={btnSecondary}>Cancel</button>
              </div>
            </div>
          )}
        </div>

        {/* Right: Knowledge Points */}
        <div style={{ ...panelStyle, borderRight: 'none', flex: 2 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
            <strong>{searchResults !== null ? 'Search Results' : 'Knowledge Points'}</strong>
            <button onClick={() => openKpForm()} disabled={!selectedChapter && searchResults === null}
              style={{ ...(!selectedChapter && searchResults === null ? btnDisabled : btnPrimary), padding: '0.25rem 0.5rem', fontSize: '0.8rem' }}>+ Add</button>
          </div>
          {!selectedChapter && searchResults === null && <div style={{ color: '#999', textAlign: 'center', padding: '2rem 0' }}>Select a chapter</div>}
          {kpsLoading && <LoadingSpinner message="Loading knowledge points..." />}
          {kpsError && <ErrorMessage message={kpsError} />}
          {!kpsLoading && !kpsError && displayKps.length === 0 && selectedChapter && (
            <EmptyState message="No knowledge points" actionLabel="Create Knowledge Point" onAction={() => openKpForm()} />
          )}
          {displayKps.map(kp => (
            <div key={kp.id} style={{ padding: '0.75rem', marginBottom: '0.5rem', border: '1px solid #eee', borderRadius: 4 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <h4 style={{ margin: 0 }}>{kp.title}</h4>
                <div style={{ display: 'flex', gap: '0.25rem' }}>
                  <button onClick={() => openKpForm(kp)} style={{ ...btnSecondary, padding: '0.2rem 0.5rem', fontSize: '0.75rem' }}>Edit</button>
                  <button onClick={() => handleExport(kp.id)} style={{ ...btnSecondary, padding: '0.2rem 0.5rem', fontSize: '0.75rem' }}>Export</button>
                </div>
              </div>
              <p style={{ margin: '0.5rem 0', color: '#555', fontSize: '0.9rem', whiteSpace: 'pre-wrap', maxHeight: 100, overflow: 'hidden' }}>{kp.content}</p>
              {kp.tags && kp.tags.length > 0 && (
                <div style={{ display: 'flex', gap: '0.25rem', flexWrap: 'wrap' }}>
                  {kp.tags.map((tag, i) => (
                    <span key={i} style={{ padding: '0.1rem 0.5rem', backgroundColor: '#e3f2fd', borderRadius: 12, fontSize: '0.75rem', color: '#1565c0' }}>{tag}</span>
                  ))}
                </div>
              )}
            </div>
          ))}
          {displayKps.length > 0 && (
            <Pagination page={displayPage} pageSize={20} total={displayTotal}
              onPageChange={p => searchResults !== null ? handleSearch(p) : fetchKps(p)} />
          )}
        </div>
      </div>

      {/* Import Modal */}
      {showImportModal && (
        <div style={{ position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 999 }} onClick={() => setShowImportModal(false)}>
          <div style={{ backgroundColor: '#fff', borderRadius: 8, width: 480, maxWidth: '90vw', boxShadow: '0 4px 24px rgba(0,0,0,0.2)', padding: '1.5rem' }} onClick={e => e.stopPropagation()}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
              <h3 style={{ margin: 0 }}>Import Content</h3>
              <button onClick={() => setShowImportModal(false)} style={{ background: 'none', border: 'none', fontSize: '1.5rem', cursor: 'pointer', color: '#666' }}>&times;</button>
            </div>
            {importFormErr && <div style={{ color: '#dc3545', marginBottom: '0.75rem', fontSize: '0.85rem' }}>{importFormErr}</div>}
            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Category *</label>
              <input value={importForm.category} onChange={e => setImportForm({ ...importForm, category: e.target.value })} placeholder="e.g. Pharmacology" style={inputStyle} />
            </div>
            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Title *</label>
              <input value={importForm.title} onChange={e => setImportForm({ ...importForm, title: e.target.value })} placeholder="Document title" style={inputStyle} />
            </div>
            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Chapter ID *</label>
              <input value={importForm.chapter_id} onChange={e => setImportForm({ ...importForm, chapter_id: e.target.value })} placeholder={selectedChapter?.id || 'Select a chapter first, or paste ID here'} style={inputStyle} />
            </div>
            <div style={{ marginBottom: '1rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>File * (.md, .html)</label>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <button onClick={() => fileInputRef.current?.click()} style={btnSecondary} type="button">Choose File</button>
                <span style={{ fontSize: '0.85rem', color: '#555' }}>{importFile ? importFile.name : 'No file chosen'}</span>
              </div>
            </div>
            <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
              <button onClick={() => setShowImportModal(false)} style={btnSecondary}>Cancel</button>
              <button onClick={handleImportSubmit} disabled={importLoading} style={importLoading ? btnDisabled : btnPrimary}>
                {importLoading ? 'Importing...' : 'Import'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* KP Form Modal */}
      {showKpForm && (
        <div style={{ position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 999 }} onClick={() => setShowKpForm(false)}>
          <div style={{ backgroundColor: '#fff', borderRadius: 8, width: 600, maxWidth: '90vw', maxHeight: '90vh', overflow: 'auto', boxShadow: '0 4px 24px rgba(0,0,0,0.2)' }} onClick={e => e.stopPropagation()}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '1rem 1.5rem', borderBottom: '1px solid #eee' }}>
              <h3 style={{ margin: 0 }}>{editingKp ? 'Edit Knowledge Point' : 'New Knowledge Point'}</h3>
              <button onClick={() => setShowKpForm(false)} style={{ background: 'none', border: 'none', fontSize: '1.5rem', cursor: 'pointer', color: '#666' }}>&times;</button>
            </div>
            <div style={{ padding: '1.5rem' }}>
              {kpFormErr && <div style={{ color: '#dc3545', marginBottom: '0.75rem', fontSize: '0.85rem' }}>{kpFormErr}</div>}
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Title *</label>
              <input value={kpForm.title} onChange={e => setKpForm({ ...kpForm, title: e.target.value })} style={{ ...inputStyle, marginBottom: '0.75rem' }} />
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Content (Markdown) *</label>
              <textarea value={kpForm.content} onChange={e => setKpForm({ ...kpForm, content: e.target.value })}
                rows={10} style={{ ...inputStyle, marginBottom: '0.75rem', fontFamily: 'monospace', resize: 'vertical' }} />
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Tags (comma-separated)</label>
              <input value={kpForm.tags} onChange={e => setKpForm({ ...kpForm, tags: e.target.value })} placeholder="tag1, tag2, tag3" style={{ ...inputStyle, marginBottom: '0.75rem' }} />
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Classifications (JSON)</label>
              <textarea value={kpForm.classifications} onChange={e => setKpForm({ ...kpForm, classifications: e.target.value })}
                rows={3} style={{ ...inputStyle, marginBottom: '1rem', fontFamily: 'monospace', resize: 'vertical' }} />
              <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
                <button onClick={() => setShowKpForm(false)} style={btnSecondary}>Cancel</button>
                <button onClick={handleKpSubmit} disabled={kpSubmitting} style={kpSubmitting ? btnDisabled : btnPrimary}>
                  {kpSubmitting ? 'Saving...' : (editingKp ? 'Update' : 'Create')}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default LearningPage;

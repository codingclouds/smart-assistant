interface FileArtifactCardProps {
  filePath: string;
  fileName: string;
  byteCount: number;
  workspaceId: string;
  onPreview?: (workspaceId: string, filePath: string, fileName: string) => void;
}

/** 文件图标映射 */
const FILE_ICONS: Record<string, string> = {
  '.md': '📝', '.markdown': '📝',
  '.txt': '📄', '.log': '📄',
  '.json': '📋', '.xml': '📋',
  '.ts': '📘', '.tsx': '📘', '.js': '📒', '.jsx': '📒',
  '.go': '🐹', '.py': '🐍', '.java': '☕', '.rs': '🦀',
  '.css': '🎨', '.html': '🌐', '.htm': '🌐',
  '.yml': '⚙️', '.yaml': '⚙️', '.toml': '⚙️',
  '.png': '🖼️', '.jpg': '🖼️', '.jpeg': '🖼️', '.gif': '🖼️',
  '.svg': '🖼️', '.webp': '🖼️',
  '.pdf': '📕',
  '.zip': '📦', '.tar': '📦', '.gz': '📦',
  '.sh': '💻', '.bash': '💻',
  '.csv': '📊',
  '.docx': '📄', '.doc': '📄', '.xlsx': '📊', '.pptx': '📊',
};

function getFileIcon(name: string): string {
  const ext = name.slice(name.lastIndexOf('.')).toLowerCase();
  return FILE_ICONS[ext] || '📄';
}

function formatSize(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes} B`;
}

export default function FileArtifactCard({
  filePath,
  fileName,
  byteCount,
  workspaceId,
  onPreview,
}: FileArtifactCardProps) {
  const icon = getFileIcon(fileName);

  const handleClick = () => {
    onPreview?.(workspaceId, filePath, fileName);
  };

  return (
    <div
      onClick={handleClick}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          handleClick();
        }
      }}
      title={`点击预览 ${fileName}`}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        marginTop: 8,
        marginBottom: 8,
        padding: '12px 14px',
        border: '1px solid var(--color-border-light)',
        borderRadius: 'var(--radius-md)',
        background: 'color-mix(in srgb, var(--color-success) 5%, var(--color-bg-secondary))',
        cursor: 'pointer',
        transition: 'background 0.15s, border-color 0.15s',
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.background = 'color-mix(in srgb, var(--color-success) 10%, var(--color-bg-hover))';
        e.currentTarget.style.borderColor = 'var(--color-accent)';
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.background = 'color-mix(in srgb, var(--color-success) 5%, var(--color-bg-secondary))';
        e.currentTarget.style.borderColor = 'var(--color-border-light)';
      }}
    >
      <span style={{ fontSize: 24, flexShrink: 0 }}>{icon}</span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            fontWeight: 600,
            fontSize: 13,
            color: 'var(--color-text-primary)',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {fileName}
        </div>
        <div
          style={{
            fontSize: 11,
            color: 'var(--color-text-muted)',
            marginTop: 1,
          }}
        >
          {formatSize(byteCount)} · {filePath}
        </div>
      </div>
      {/* 预览箭头指示 */}
      <svg
        width="16" height="16" viewBox="0 0 16 16" fill="none"
        style={{ flexShrink: 0, color: 'var(--color-text-muted)' }}
      >
        <path
          d="M6 4L10 8L6 12"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </div>
  );
}
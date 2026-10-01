export function AnimationBadge({ label }: { label?: string }) {
  if (!label) return null;
  return (
    <span
      className="media-animation-badge"
      title={`Animated image: ${label}`}
      aria-label={`Animated image: ${label}`}
    >
      {label}
    </span>
  );
}

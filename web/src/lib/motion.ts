// Shared motion primitives for the console. Kept dependency-light: the
// `motion` (Framer Motion) package is already installed, so we centralise the
// easing and the small set of variants used across screens here.

// A confident ease-out shared across the console.
export const EASE_OUT: [number, number, number, number] = [0.22, 1, 0.36, 1];

// A card lifting in on mount. Used as a child of a stagger container.
export const cardVariants = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.4, ease: EASE_OUT } },
};

// Container that staggers its children's entrance.
export const staggerContainer = {
  hidden: {},
  show: { transition: { staggerChildren: 0.07, delayChildren: 0.04 } },
};

export const trapFocus = (element: HTMLElement) => {
  const focusableElements = element.querySelectorAll(
    'a[href], button, textarea, input[type="text"], input[type="radio"], input[type="checkbox"], select'
  );
  const firstElement = focusableElements[0] as HTMLElement;
  const lastElement = focusableElements[focusableElements.length - 1] as HTMLElement;

  const handleTab = (e: KeyboardEvent) => {
    if (e.key === 'Tab') {
      if (e.shiftKey) {
        if (document.activeElement === firstElement) {
          e.preventDefault();
          lastElement.focus();
        }
      } else {
        if (document.activeElement === lastElement) {
          e.preventDefault();
          firstElement.focus();
        }
      }
    }
  };

  element.addEventListener('keydown', handleTab);
  return () => element.removeEventListener('keydown', handleTab);
};

export const announceRoute = (title: string) => {
  const liveRegion = document.getElementById('a11y-live-region');
  if (liveRegion) {
    liveRegion.textContent = `Navigated to ${title}`;
  }
};

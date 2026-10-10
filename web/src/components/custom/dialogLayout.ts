/**
 * Class names for a dialog whose header and footer stay put while only the
 * middle scrolls, so the title and the Cancel / Confirm buttons are always
 * reachable on a short window. Use them with the shadcn parts:
 *
 *   <DialogContent className={dialogLayout.content}>
 *     <DialogHeader className={dialogLayout.header}>...</DialogHeader>
 *     <form className={dialogLayout.form}>
 *       <div className={dialogLayout.body}>fields</div>
 *       <DialogFooter className={dialogLayout.footer}>buttons</DialogFooter>
 *     </form>
 *   </DialogContent>
 *
 * DialogContent pads itself, so the padding moves onto the three regions: that
 * is what lets the scrolling one reach the dialog's edges.
 */
export const dialogLayout = {
  content: "flex max-h-[85vh] flex-col gap-0 p-0",
  header: "shrink-0 px-6 pt-6 pb-4",
  form: "flex min-h-0 flex-1 flex-col",
  // The vertical padding leaves room for the focus ring of the first and last field.
  body: "flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-1",
  footer: "shrink-0 px-6 pt-4 pb-6",
  // An error belongs next to the buttons, not at the bottom of the scroll.
  error: "shrink-0 px-6 pt-3 text-sm font-medium text-destructive",
} as const

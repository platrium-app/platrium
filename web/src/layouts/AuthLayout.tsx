import { cn } from "cn"
import { FieldDescription } from "@/components/ui/field"
import PlatriumLogo from "@/assets/PlatriumLogo"

export function AuthLayout({
  className,
  children,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div className="flex min-h-svh flex-col items-center justify-center gap-6 bg-muted p-6 md:p-10">
      <div className="flex w-full max-w-sm flex-col gap-6">
        <a href="#" className="flex items-center gap-1 self-center font-medium">
          <div className="flex h-6 w-6 items-center justify-center">
            <PlatriumLogo />
          </div>
          Platrium
        </a>
        <div className={cn("flex flex-col gap-6", className)} {...props}>
          {children}
        </div>
        <FieldDescription className="px-6 text-center">
          By clicking continue, you agree to our <a href="#">Terms of Service</a>{" "}
          and <a href="#">Privacy Policy</a>.
        </FieldDescription>
      </div>
    </div>
  )
}


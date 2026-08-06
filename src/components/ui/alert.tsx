import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { AlertCircle } from "lucide-react"

import { cn } from "@/lib/utils"

const alertVariants = cva(
  "flex items-start gap-2 rounded-md border px-3 py-2 text-sm",
  {
    variants: {
      variant: {
        destructive: "border-destructive/30 bg-destructive/10 text-destructive",
        warning: "border-warning/30 bg-warning/10 text-warning",
        success: "border-success/30 bg-success/10 text-success",
      },
    },
    defaultVariants: {
      variant: "destructive",
    },
  }
)

export interface AlertProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof alertVariants> {
  icon?: React.ReactNode
}

function Alert({ className, variant, icon, children, ...props }: AlertProps) {
  return (
    <div role="alert" className={cn(alertVariants({ variant }), className)} {...props}>
      {icon ?? <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />}
      <div className="min-w-0">{children}</div>
    </div>
  )
}

export { Alert, alertVariants }

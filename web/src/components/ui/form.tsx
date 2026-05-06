import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import {
  Controller,
  FormProvider,
  useFormContext,
  type ControllerProps,
  type FieldPath,
  type FieldValues,
} from 'react-hook-form';

import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

type FormFieldContextValue<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> = {
  name: TName;
};

const FormFieldContext = React.createContext<FormFieldContextValue>({} as FormFieldContextValue);
const FormItemContext = React.createContext<{ id: string }>({ id: '' });

// Form 表示 react-hook-form 的上下文壳，方便表单子组件拿到同一份状态。
export const Form = FormProvider;

// FormField 用来把字段名传给后续标签、输入框和报错区，减少重复样板代码。
export function FormField<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({ ...props }: ControllerProps<TFieldValues, TName>) {
  return (
    <FormFieldContext.Provider value={{ name: props.name }}>
      <Controller {...props} />
    </FormFieldContext.Provider>
  );
}

// useFormField 用来集中组装字段关联信息，确保 label、输入框和错误提示都能正确绑定。
export function useFormField() {
  const fieldContext = React.useContext(FormFieldContext);
  const itemContext = React.useContext(FormItemContext);
  const { getFieldState, formState } = useFormContext();

  const fieldState = getFieldState(fieldContext.name, formState);
  const { id } = itemContext;

  return {
    id,
    name: fieldContext.name,
    formItemId: `${id}-form-item`,
    formDescriptionId: `${id}-form-item-description`,
    formMessageId: `${id}-form-item-message`,
    ...fieldState,
  };
}

// FormItem 表示单个表单项容器，让输入框、标签和错误提示保持统一间距。
export const FormItem = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(({ className, ...props }, ref) => {
  const id = React.useId();

  return (
    <FormItemContext.Provider value={{ id }}>
      <div className={cn('space-y-2', className)} ref={ref} {...props} />
    </FormItemContext.Provider>
  );
});

FormItem.displayName = 'FormItem';

// FormLabel 表示表单字段标题，会在字段出错时自动切换提示颜色。
export const FormLabel = React.forwardRef<React.ElementRef<typeof Label>, React.ComponentPropsWithoutRef<typeof Label>>(
  ({ className, ...props }, ref) => {
    const { error, formItemId } = useFormField();

    return <Label className={cn(error ? 'text-rose-700' : '', className)} htmlFor={formItemId} ref={ref} {...props} />;
  },
);

FormLabel.displayName = 'FormLabel';

// FormControl 用来把可交互控件和字段状态绑在一起，补齐无障碍属性。
export const FormControl = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(({ ...props }, ref) => {
  const { error, formDescriptionId, formItemId, formMessageId } = useFormField();

  return (
    <Slot
      aria-describedby={error ? `${formDescriptionId} ${formMessageId}` : formDescriptionId}
      aria-invalid={Boolean(error)}
      id={formItemId}
      ref={ref}
      {...props}
    />
  );
});

FormControl.displayName = 'FormControl';

// FormMessage 表示字段报错区，没有错误时就不占页面空间。
export const FormMessage = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(
  ({ className, children, ...props }, ref) => {
    const { error, formMessageId } = useFormField();
    const body = error ? String(error.message ?? '') : children;

    if (!body) {
      return null;
    }

    return (
      <p className={cn('text-sm font-medium text-rose-700', className)} id={formMessageId} ref={ref} {...props}>
        {body}
      </p>
    );
  },
);

FormMessage.displayName = 'FormMessage';
